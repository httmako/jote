package jote

import (
	"log/slog"
	"os"
	"sigs.k8s.io/yaml"
)

// Panics if error is not nil. Can be used to wrap functions that only return error to automatically panic.
// Mainly used for script startups where errors are unrecoverable.
func Must(err error) {
	if err != nil {
		panic(err)
	}
}

// Can be used with value,error return functions where value is not needed and error must not be nil. For example [io/WriteString].
// Mainly used for script startups where errors are unrecoverable.
func Must2(obj any, err error) {
	if err != nil {
		panic(err)
	}
}

// The same as [Must2] but returns the first value if error is not nil. If error is nil, it panics.
func Must2r(obj any, err error) any {
	if err != nil {
		panic(err)
	}
	return obj
}

// Uses [os/ReadFile], panics on error, returns string of the file contents if error is nil.
func MustReadFile(filename string) string {
	fileBytes, err := os.ReadFile(filename)
	if err != nil { panic(err) }
	return string(fileBytes)
}

// A simple wrapper for [sigs.k8s.io/yaml] to read a yaml file from path and parse it into the 2nd argument.
// If the file read or parsing returns an error it will panic with the error.
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

// A wrapper for [log/slog] that creates a slog logger.
// If path is "stdout" it will create a stdout logger, else it will log into the path file.
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

// Same as CreateLogger but returns the loglevel to control the logger.
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

// Small wrapper to start a goroutine and defer recover.
func Go(gFunc func()) {
	defer recover()
	go gFunc()
}
