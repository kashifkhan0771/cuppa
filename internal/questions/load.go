// Package questions reads, checks and compiles the hand-written
// question YAML files. Both the datagen tool (for questions/) and the
// cuppa CLI (for a team's private packs) use it.
package questions

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/kashifkhan0771/cuppa/pkg/cuppa"
)

// retiredFileName holds IDs of removed questions so they are never reused.
const retiredFileName = "_retired.yaml"

// File is one questions/<category>.yaml file as people write it.
type File struct {
	Path      string           `yaml:"-"`
	Category  string           `yaml:"category"`
	Name      string           `yaml:"name"`
	Prefix    string           `yaml:"prefix"`
	Questions []cuppa.Question `yaml:"questions"`
}

type retiredFile struct {
	Retired []Retired `yaml:"retired"`
}

type Retired struct {
	ID     string `yaml:"id"`
	Reason string `yaml:"reason"`
}

// LoadDir reads every category file in dir, plus the retired list.
// A missing dir gives no files and no error.
func LoadDir(dir string) ([]File, []Retired, error) {
	// Glob returns names in sorted order, so output is always the same
	paths, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, nil, err
	}

	var files []File
	var retired []Retired
	for _, path := range paths {
		if filepath.Base(path) == retiredFileName {
			var r retiredFile
			if err := decodeYAML(path, &r); err != nil {
				return nil, nil, err
			}
			retired = r.Retired
			continue
		}

		f, err := LoadFile(path)
		if err != nil {
			return nil, nil, err
		}
		files = append(files, f)
	}
	return files, retired, nil
}

// LoadFile reads one question file, such as a team's private pack.
func LoadFile(path string) (File, error) {
	var f File
	if err := decodeYAML(path, &f); err != nil {
		return File{}, err
	}
	f.Path = path

	// each question gets its category from the file it lives in
	for i := range f.Questions {
		f.Questions[i].Category = f.Category
	}
	return f, nil
}

// decodeYAML reads path into v.
// Unknown keys are an error, so typos like "tagz:" get caught.
func decodeYAML(path string, v any) error {
	data, err := os.ReadFile(path) // #nosec G304 -- path comes from our own glob
	if err != nil {
		return err
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(v); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// ReadWordList reads a text file with one entry per line.
// Blank lines and lines starting with # are skipped.
// Entries are stored lowercase.
func ReadWordList(path string) (map[string]bool, error) {
	f, err := os.Open(path) // #nosec G304 -- path comes from a command flag
	if err != nil {
		return nil, err
	}
	defer f.Close()

	words := map[string]bool{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		words[strings.ToLower(line)] = true
	}
	return words, scanner.Err()
}

// AllQuestions flattens the questions from every file into one list.
func AllQuestions(files []File) []cuppa.Question {
	var all []cuppa.Question
	for _, f := range files {
		all = append(all, f.Questions...)
	}
	return all
}
