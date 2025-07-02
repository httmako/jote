package jote

import (
	"log/slog"
	"os"
	"sigs.k8s.io/yaml"
)

func Must(err error) {
	if err != nil {
		panic(err)
	}
}

func Must2(obj any, err error) {
	if err != nil {
		panic(err)
	}
}

func Must2r(obj any, err error) any {
	if err != nil {
		panic(err)
	}
	return obj
}

func ReadConfigYAML(path string, config any) {
	configfile, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal(configfile, &config)
	if err != nil {
		panic(err)
	}
}

func CreateLogger(path string) *slog.Logger {
	if path == "stdout" {
		return slog.New(slog.NewTextHandler(os.Stdout, nil))
	} else {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			panic(err)
		}
		return slog.New(slog.NewTextHandler(f, nil))
	}
}

func CreateLoggerWithLevel(path string) (*slog.Logger, *slog.LevelVar) {
	loglvl := new(slog.LevelVar)
	logHO := &slog.HandlerOptions{Level: loglvl}

	if path == "stdout" {
		return slog.New(slog.NewTextHandler(os.Stdout, logHO)), loglvl
	} else {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			panic(err)
		}
		return slog.New(slog.NewTextHandler(f, logHO)), loglvl
	}
}

func Go(gFunc func()) {
	defer recover()
	go gFunc()
}
