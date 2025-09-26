package jote

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
)

// Object to query the Midadol config server
type Mid struct {
	// Target URL, where the Midadol server is running, e.g. http://localhost:5911
	URL string
	// Application name
	App string
	// Environment name, this is optional as "" is a valid environment
	Env string
}

// Shortcut to create a Mid object where the env is "" and the app is the executable name taken from os.Args[0].
// If the URL is "" it will use "http://localhost:5911" as the URL
func NewMidadolSimple(url string) Mid {
	if url == "" {
		url = "http://localhost:5911"
	}
	return Mid{
		URL: url,
		App: filepath.Base(os.Args[0]),
	}
}

// Creates a Mid object for querying the Midadol config server
func NewMidadol(url string, app string, env string) Mid {
	return Mid{
		URL: url,
		App: app,
		Env: env,
	}
}

// Gets the config value as a string. It panics if the response code of the Midadol server is 404.
// If the request of http.Get creates a non-nil error it also panics.
// Nothing gets cached, so every Get* sends an http request to the Midadol server.
// The choice to panic was deliberate, because config values should be loaded on startup.
func (mid *Mid) Get(key string) string {
	url, err := url.JoinPath(mid.URL, "get", mid.App, key)
	if err != nil {
		panic(fmt.Errorf("[MIDADOL] ERROR: could not join URL: %s", err))
	}
	resp, err := http.Get(url)
	if err != nil {
		panic(fmt.Errorf("[MIDADOL] ERROR: http.Get error: %s", err))
	}
	if resp.StatusCode == 404 {
		panic("[MIDADOL] ERROR: value for key not found, received 404")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(fmt.Errorf("[MIDADOL] ERROR: io.ReadAll error: %s", err))
	}
	return string(body)
}

// Uses [Get] internally and tries to convert it to an int via [strconv.Atoi].
// It panics if the conversion creates an error.
func (mid *Mid) GetInt(key string) int {
	num, err := strconv.Atoi(mid.Get(key))
	if err != nil {
		panic(fmt.Errorf("[MIDADOL] ERROR: value of key is not an int: %s", err))
	}
	return num
}

// Uses [Get] internally and returns true if the value retrieved is any of the following:
// 1, true, TRUE, True
// Otherwise it returns false.
func (mid *Mid) GetBool(key string) bool {
	val := mid.Get(key)
	if val == "1" || val == "true" || val == "TRUE" || val == "True" {
		return true
	}
	return false
}
