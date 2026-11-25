package pipeline

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// builtins are the variables TRUSS sets in every run (spec 026). A flow
// file, a project file or a secret may not set them. TRUSS_DATA_DIR and
// TRUSS_CACHE_DIR are not here: a file could set them before v0.3.0.
var builtins = map[string]bool{
	"TRUSS_PROJECT_DIR": true, "TRUSS_FLOW": true, "TRUSS_PIPELINE": true, "TRUSS_JOB": true,
	"TRUSS_STEP": true, "TRUSS_ATTEMPT": true, "TRUSS_RUN_ID": true, "TRUSS_TRIGGER": true,
	"TRUSS_TEMP_DIR": true, "TRUSS_OS": true, "TRUSS_ARCH": true,
}

// Reserved reports whether name is a built-in variable, which a flow file
// may not set. Case does not matter: Windows folds it.
func Reserved(name string) bool {
	return builtins[strings.ToUpper(name)]
}

// CheckTemplate reports a syntax error in a value that may hold ${NAME}
// references: an unclosed ${ or a name that is not a variable name. "$$" is a
// literal "$"; a "$" before anything else is kept as written.
func CheckTemplate(value string) error {
	_, err := expand(value, func(string) (string, error) { return "", nil })
	return err
}

// Expand replaces ${NAME} in value with lookup(NAME) and "$$" with "$".
func Expand(value string, lookup func(string) string) (string, error) {
	return expand(value, func(name string) (string, error) { return lookup(name), nil })
}

func expand(value string, lookup func(string) (string, error)) (string, error) {
	if !strings.Contains(value, "$") {
		return value, nil
	}
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c != '$' || i+1 == len(value) {
			b.WriteByte(c)
			continue
		}
		switch value[i+1] {
		case '$':
			b.WriteByte('$')
			i++
		case '{':
			end := strings.IndexByte(value[i+2:], '}')
			if end < 0 {
				return "", errors.New("a ${ has no closing }")
			}
			name := value[i+2 : i+2+end]
			if !envName.MatchString(name) {
				return "", fmt.Errorf("${%s} is not a variable name (letters, digits and '_', not starting with a digit)", name)
			}
			v, err := lookup(name)
			if err != nil {
				return "", err
			}
			b.WriteString(v)
			i += 2 + end
		default:
			b.WriteByte(c)
		}
	}
	return b.String(), nil
}

// ExpandLayers resolves the variables of a step, layer by layer from the
// lowest (project) to the highest (step). Inside a layer a value may use the
// other names of the same layer, in any order; ${NAME} in the value of NAME
// itself, and any name the layer does not set, is looked up in what the
// layers below and base give. A cycle is an error. The result holds every
// name the layers set, expanded.
func ExpandLayers(base func(string) string, layers ...map[string]string) (map[string]string, error) {
	out := map[string]string{}
	below := func(name string) string {
		if v, ok := out[name]; ok {
			return v
		}
		return base(name)
	}
	for _, layer := range layers {
		if len(layer) == 0 {
			continue
		}
		done := map[string]string{}
		state := map[string]int{} // 1: in progress, 2: done
		var resolve func(name string) (string, error)
		resolve = func(name string) (string, error) {
			if v, ok := done[name]; ok {
				return v, nil
			}
			if state[name] == 1 {
				return "", fmt.Errorf("the variables %s refer to each other in a loop", name)
			}
			state[name] = 1
			v, err := expand(layer[name], func(ref string) (string, error) {
				if ref != name {
					if _, ok := layer[ref]; ok {
						return resolve(ref)
					}
				}
				return below(ref), nil
			})
			if err != nil {
				return "", err
			}
			state[name] = 2
			done[name] = v
			return v, nil
		}
		names := make([]string, 0, len(layer))
		for name := range layer {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if _, err := resolve(name); err != nil {
				return nil, err
			}
		}
		for name, v := range done {
			out[name] = v
		}
	}
	return out, nil
}
