package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"
	"gopkg.in/yaml.v2"
	"github.com/urfave/cli"
	"errors"
)

type appYaml struct {
	Env_variables map[string]string
	Includes []string
}


type gaeenv struct {
	silent bool
	force bool
	vars *sync.Map
	files *sync.Map
}

func New(silent bool, force bool) *gaeenv {
	return &gaeenv{
		silent: silent,
		force: force,
		vars: &sync.Map{},
		files: &sync.Map{},
	}
}

func (g *gaeenv) verifyFile(parent string, path string) error {
	if oldParent, ok := g.files.Load(path); ok {
		return errors.New(fmt.Sprintf("Duplicate file: %s found in %s, was already loaded in %s", path, parent, oldParent))
	}
	if _, err := os.Stat(path); err == nil {
		g.files.Store(path, parent)
		return nil
	} else if os.IsNotExist(err) {
		return err
	} else {
		return errors.New(fmt.Sprintf("Unable to find %s, file seems to exist, but errors on read"))
	}

}

func (g *gaeenv) addVars(vars map[string]string) error {
	for key, value := range vars {
		g.vars.Store(key, value)
	}
	return nil
}

func (g *gaeenv) AddFile(parent string, path string) error {
	err := g.verifyFile(parent, path)
	if err != nil {
		return g.handleError("AddFile", err)
	}

	content, err := ioutil.ReadFile(path)
	if err != nil {
		return g.handleError("AddFile", err)
	}
	thisYaml := &appYaml{}

	err = yaml.Unmarshal(content, &thisYaml)
	if err != nil {
		return g.handleError("AddFile", err)
	}

	relDir := filepath.Dir(path)
	for _, file := range thisYaml.Includes {
		if err := g.AddFile(path, filepath.Join(relDir, file)); err != nil {
			return g.handleError("AddFile", err)
		}
	}

	err = g.addVars(thisYaml.Env_variables)
	if err != nil {
		return g.handleError("AddFile", err)
	}

	return nil
}

func (g *gaeenv) handleError(fun string, err error) error {
	if !g.silent {
		_, _ = fmt.Fprintf(os.Stderr, "Error in %s, %v\n", fun, err)
	}
	if !g.force {
		os.Exit(1)
	}
	return nil
}

func (g *gaeenv) printVars() {
	g.vars.Range(func (key interface{}, value interface{}) bool {
		fmt.Printf("export %s=\"%s\"\n", key, value)
		return true
	})
}

func main() {
	app := cli.NewApp()
	app.Name = "gaeenv"
	app.Usage = "dumps app.yaml env variables to source in scripts/startup"
	app.Version = "0.0.1"

	var yamlFile string
	var silent bool
	var force bool

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "config, c",
			Value: "app.yaml",
			Usage: "Export env from `app.yaml`",
			Destination: &yamlFile,
		},
		cli.BoolFlag{
			Name: "silent, s",
			Usage: "Don't print errors",
			Destination: &silent,
		},
		cli.BoolFlag{
			Name: "force, f",
			Usage: "Keep processing regardless of errors",
			Destination: &force,
		},
	}

	app.Action = func(c *cli.Context) error {
		gaeEnv := New(silent, force)
		err := gaeEnv.AddFile("", yamlFile)
		if err != nil {
			return err
		}
		gaeEnv.printVars()
		return nil
	}


	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

