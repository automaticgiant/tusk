package appcli

import (
	"io/ioutil"
	"os"
	"regexp"

	"github.com/pkg/errors"
	"github.com/urfave/cli"

	"gitlab.com/rliebz/tusk/config"
)

// NewBaseApp creates a basic cli.App with top-level flags.
func NewBaseApp() *cli.App {
	app := cli.NewApp()
	app.Usage = "a task runner built with simple configuration in mind"
	app.HideVersion = true
	app.HideHelp = true

	app.Flags = append(app.Flags,
		cli.HelpFlag,
		cli.StringFlag{
			Name:  "file, f",
			Usage: "Set `FILE` to use as the config file",
		},
		cli.BoolFlag{
			Name:  "verbose, v",
			Usage: "Print verbose output",
		},
	)

	return app
}

// NewSilentApp creates a cli.App that will never print to stderr / stdout.
func NewSilentApp() *cli.App {
	app := NewBaseApp()
	app.Writer = ioutil.Discard
	app.ErrWriter = ioutil.Discard
	app.CommandNotFound = func(c *cli.Context, command string) {}
	return app
}

// NewFlagApp creates a cli.App that can parse flags.
func NewFlagApp(cfgText []byte) (*cli.App, error) {
	flagCfg, err := config.Parse(cfgText)
	if err != nil {
		return nil, err
	}

	flagApp := NewSilentApp()

	if err = addTasks(flagApp, flagCfg, createMetadataBuildCommand); err != nil {
		return nil, err
	}

	if err = flagApp.Run(os.Args); err != nil {
		return nil, err
	}

	return flagApp, nil
}

// NewApp creates a cli.App that executes tasks.
func NewApp(cfgText []byte) (*cli.App, error) {
	flagApp, err := NewFlagApp(cfgText)
	if err != nil {
		return nil, err
	}

	flags, ok := flagApp.Metadata["flagValues"].(map[string]string)
	if !ok {
		return nil, errors.New("could not read flags from metadata")
	}

	cfgText, err = interpolate(cfgText, flags)
	if err != nil {
		return nil, err
	}

	appCfg, err := config.Parse(cfgText)
	if err != nil {
		return nil, err
	}

	app := NewBaseApp()

	if err := addTasks(app, appCfg, createExecuteCommand); err != nil {
		return nil, err
	}

	copyFlags(app, flagApp)

	return app, nil
}

func interpolate(cfgText []byte, flags map[string]string) ([]byte, error) {

	for flagName, value := range flags {
		pattern := config.InterpolationPattern(flagName)
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}

		cfgText = re.ReplaceAll(cfgText, []byte(value))
	}

	return cfgText, nil
}