package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/afri-bit/truss/internal/home"
	"github.com/afri-bit/truss/internal/secrets"
)

// secretLoader returns a function that reads the secret store of the TRUSS
// home named by env. It resolves the home only when called, so a run that
// declares no secrets never needs one.
func secretLoader(env []string) func() (map[string]string, error) {
	return func() (map[string]string, error) {
		h, err := home.Resolve(env)
		if err != nil {
			return nil, err
		}
		return secrets.Open(h.Secrets()).Load()
	}
}

// secret runs `truss secret set|list|remove`. It never prints a value.
func (in invocation) secret() int {
	if len(in.args) == 0 {
		return in.usage()
	}
	sub, rest := in.args[0], in.args[1:]
	switch {
	case sub == "list" && len(rest) == 0, (sub == "set" || sub == "remove") && len(rest) == 1:
	default:
		return in.usage()
	}
	h, err := home.Resolve(in.env)
	if err != nil {
		return in.fail(err)
	}
	store := secrets.Open(h.Secrets())
	switch sub {
	case "list":
		names, err := store.Names()
		if err != nil {
			return in.fail(err)
		}
		for _, n := range names {
			fmt.Fprintln(in.stdout, n)
		}
	case "remove":
		if err := store.Remove(rest[0]); err != nil {
			return in.fail(err)
		}
		fmt.Fprintf(in.stdout, "secret %s removed\n", rest[0])
	case "set":
		name := rest[0]
		if !secrets.ValidName(name) {
			return in.fail(fmt.Errorf("%q is not a valid secret name: use letters, digits and '_', not starting with a digit", name))
		}
		value, err := in.readSecretValue(name)
		if err != nil {
			return in.fail(err)
		}
		if err := store.Set(name, value); err != nil {
			return in.fail(err)
		}
		fmt.Fprintf(in.stdout, "secret %s saved\n", name)
	}
	return 0
}

// readSecretValue reads a value from a terminal without echo, asking twice,
// or from a pipe or file as the whole input minus one trailing newline.
func (in invocation) readSecretValue(name string) (string, error) {
	if f, ok := in.stdin.(*os.File); ok && isTerminal(f) {
		first, err := readHidden(f, in.stderr, "Value for "+name+" (input is hidden): ")
		if err != nil {
			return "", err
		}
		again, err := readHidden(f, in.stderr, "Repeat the value: ")
		if err != nil {
			return "", err
		}
		if first != again {
			return "", errors.New("the two values differ; nothing was stored")
		}
		return first, nil
	}
	if in.stdin == nil {
		return "", errors.New("no value: pipe it to standard input, for example: printf %s \"$TOKEN\" | truss secret set " + name)
	}
	data, err := io.ReadAll(in.stdin)
	if err != nil {
		return "", fmt.Errorf("read the value: %w", err)
	}
	value := strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
	if value == "" {
		return "", errors.New("the value is empty; nothing was stored")
	}
	return value, nil
}

// readHidden prompts on w and reads one line from the terminal f with echo off.
func readHidden(f *os.File, w io.Writer, prompt string) (string, error) {
	restore, err := disableEcho(f)
	if err != nil {
		return "", fmt.Errorf("cannot hide the input on this terminal; pipe the value instead: %w", err)
	}
	defer restore()
	fmt.Fprint(w, prompt)
	line, err := bufio.NewReader(f).ReadString('\n')
	fmt.Fprintln(w)
	if err != nil && (!errors.Is(err, io.EOF) || line == "") {
		return "", fmt.Errorf("read the value: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}
