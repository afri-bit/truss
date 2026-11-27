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

// MaxExpanded bounds an expanded value (spec 026): a few lines that double
// each other would otherwise grow past any memory.
const MaxExpanded = 128 << 10

// ErrTooLong is returned when an expanded value grows past MaxExpanded.
var ErrTooLong = fmt.Errorf("grows past %d KiB when its ${...} are expanded", MaxExpanded>>10)

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
				// The text inside ${} is part of a value: never quote it.
				return "", errors.New("a ${...} holds something that is not a variable name (letters, digits and '_', not starting with a digit)")
			}
			v, err := lookup(name)
			if err != nil {
				return "", err
			}
			if b.Len()+len(v) > MaxExpanded {
				return "", ErrTooLong
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
// name the layers set, expanded, under the spelling of the highest layer that
// sets it. On a host that ignores the case of names (Windows), names that
// differ only in case are one variable.
func ExpandLayers(base func(string) string, layers ...map[string]string) (map[string]string, error) {
	return expandLayers(foldEnvNames, base, layers...)
}

type entry struct{ name, value string }

func expandLayers(fold bool, base func(string) string, layers ...map[string]string) (map[string]string, error) {
	out := map[string]entry{} // by envKey
	below := func(name string) string {
		if e, ok := out[envKey(fold, name)]; ok {
			return e.value
		}
		return base(name)
	}
	for _, layer := range layers {
		if len(layer) == 0 {
			continue
		}
		// One layer may hold a name twice in different case only on a host
		// that keeps case; on Windows the later spelling, sorted, wins.
		names := make([]string, 0, len(layer))
		for name := range layer {
			names = append(names, name)
		}
		sort.Strings(names)
		raw := map[string]entry{}
		for _, name := range names {
			raw[envKey(fold, name)] = entry{name, layer[name]}
		}
		done := map[string]string{}
		state := map[string]int{} // 1: in progress, 2: done
		var resolve func(key string) (string, error)
		resolve = func(key string) (string, error) {
			if v, ok := done[key]; ok {
				return v, nil
			}
			if state[key] == 1 {
				return "", fmt.Errorf("the variables %s refer to each other in a loop", raw[key].name)
			}
			state[key] = 1
			v, err := expand(raw[key].value, func(ref string) (string, error) {
				rk := envKey(fold, ref)
				if rk != key {
					if _, ok := raw[rk]; ok {
						return resolve(rk)
					}
				}
				return below(ref), nil
			})
			if errors.Is(err, ErrTooLong) {
				return "", fmt.Errorf("%s %w", raw[key].name, err)
			}
			if err != nil {
				return "", err
			}
			state[key] = 2
			done[key] = v
			return v, nil
		}
		keys := make([]string, 0, len(raw))
		for k := range raw {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if _, err := resolve(k); err != nil {
				return nil, err
			}
		}
		for k, v := range done {
			out[k] = entry{raw[k].name, v}
		}
	}
	result := make(map[string]string, len(out))
	for _, e := range out {
		result[e.name] = e.value
	}
	return result, nil
}
