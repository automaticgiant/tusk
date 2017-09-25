package appcli

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/urfave/cli"

	"github.com/rliebz/tusk/config"
)

// newBaseApp creates a basic cli.App with top-level flags.
func newBaseApp() *cli.App {
	app := cli.NewApp()
	app.Usage = "a task runner built with simplicity in mind"
	app.HideVersion = true
	app.HideHelp = true

	app.Flags = append(app.Flags,
		cli.BoolFlag{
			Name:  "h, help",
			Usage: "Show help",
		},
		cli.StringFlag{
			Name:  "f, file",
			Usage: "Set `file` to use as the config file",
		},
		cli.BoolFlag{
			Name:  "q, quiet",
			Usage: "Only print command output and application errors",
		},
		cli.BoolFlag{
			Name:  "v, verbose",
			Usage: "Print verbose output",
		},
		cli.BoolFlag{
			Name:  "V, version",
			Usage: "Print version and exit",
		},
	)

	return app
}

// newSilentApp creates a cli.App that will never print to stderr / stdout.
func newSilentApp() *cli.App {
	app := newBaseApp()
	app.Writer = ioutil.Discard
	app.ErrWriter = ioutil.Discard
	app.CommandNotFound = func(c *cli.Context, command string) {}
	return app
}

// newFlagApp creates a cli.App that can parse flags.
func newFlagApp(cfgText []byte) (*cli.App, error) {
	cfg, err := config.Parse(cfgText)
	if err != nil {
		return nil, err
	}

	app := newSilentApp()
	app.Metadata = map[string]interface{}{
		"argsPassed":  cli.Args{},
		"flagsPassed": make(map[string]string),
	}

	if err = addTasks(app, cfg, createMetadataBuildCommand); err != nil {
		return nil, err
	}

	return app, nil
}

// NewApp creates a cli.App that executes tasks.
func NewApp(cfgText []byte) (*cli.App, error) {
	flagApp, err := newFlagApp(cfgText)
	if err != nil {
		return nil, err
	}

	if err = flagApp.Run(os.Args); err != nil {
		return nil, err
	}

	argsPassed, ok := flagApp.Metadata["argsPassed"].(cli.Args)
	if !ok {
		return nil, fmt.Errorf("args passed are of wrong type, %#v", argsPassed)
	}

	flagsPassed, ok := flagApp.Metadata["flagsPassed"].(map[string]string)
	if !ok {
		return nil, fmt.Errorf("flags passed are of wrong type, %#v", flagsPassed)
	}

	passed := config.Passed{
		Args:  argsPassed,
		Flags: flagsPassed,
	}

	var taskName string
	command, ok := flagApp.Metadata["command"].(*cli.Command)
	if ok {
		taskName = command.Name
	}

	cfgText, flags, err := config.Interpolate(cfgText, passed, taskName)
	if err != nil {
		return nil, err
	}

	cfg, err := config.Parse(cfgText)
	if err != nil {
		return nil, err
	}

	for _, t := range cfg.Tasks {
		t.Vars = flags
	}

	app := newBaseApp()

	if err := addTasks(app, cfg, createExecuteCommand); err != nil {
		return nil, err
	}

	copyFlags(app, flagApp)

	return app, nil
}

// GetConfigMetadata returns a metadata object based on global options passed.
func GetConfigMetadata(args []string) (*config.Metadata, error) {

	var err error

	app := newSilentApp()
	metadata := new(config.Metadata)

	// To prevent app from exiting, the app.Action must return nil on error.
	// The enclosing function will still return the error.
	app.Action = func(c *cli.Context) error {

		fullPath := c.String("file")
		if fullPath != "" {
			metadata.CfgText, err = ioutil.ReadFile(fullPath)
			if err != nil {
				return nil
			}
		} else {
			var found bool
			fullPath, found, err = config.SearchForFile()
			if err != nil {
				return nil
			}

			if found {
				metadata.CfgText, err = ioutil.ReadFile(fullPath)
				if err != nil {
					return nil
				}
			}
		}

		metadata.Directory = filepath.Dir(fullPath)
		metadata.RunVersion = c.Bool("version")
		metadata.Quiet = c.Bool("quiet")
		metadata.Verbose = c.Bool("verbose")
		return err
	}

	// Other run errors should not shadow app.Action errors.
	if runErr := app.Run(args); runErr != nil {
		return nil, runErr
	}

	return metadata, err
}
