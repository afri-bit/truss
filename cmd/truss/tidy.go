package main

import (
	"fmt"
	"sort"
	"strings"
)

// commands is every first word truss accepts, for "did you mean" and for
// shell completion. Keep it in step with the switch in RunWithInput.
var commands = []string{
	"cache", "cancel", "completion", "daemon", "dashboard", "desktop", "env", "flows", "help",
	"logs", "pause", "pipelines", "project", "projects", "register", "resume", "run", "runs",
	"schedules", "secret", "status", "trigger", "unregister", "validate", "version",
}

// subcommands are the second words of commands that have them.
var subcommands = map[string][]string{
	"cache":      {"clear"},
	"completion": {"bash", "zsh", "fish", "powershell"},
	"daemon":     {"start", "stop", "status", "run", "install", "uninstall", "logs"},
	"flows":      {"list", "show", "graph"},
	"pipelines":  {"list", "show"},
	"project":    {"show"},
	"projects":   {"list"},
	"runs":       {"show"},
	"schedules":  {"list"},
	"secret":     {"set", "list", "remove"},
}

// suggest returns the known word closest to word, or "" when none is close
// enough to be a likely typo: at most one edit for short words, two for longer.
func suggest(word string, known []string) string {
	limit := 1
	if len(word) > 5 {
		limit = 2
	}
	best, bestDist := "", limit+1
	for _, k := range known {
		if d := editDistance(strings.ToLower(word), k); d < bestDist {
			best, bestDist = k, d
		}
	}
	return best
}

// editDistance is the Levenshtein distance between a and b.
func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

// unknownCommand explains a first word truss does not know.
func (in invocation) unknownCommand(word string) int {
	fmt.Fprintf(in.stderr, "error: unknown command %q\n", word)
	if s := suggest(word, commands); s != "" {
		fmt.Fprintf(in.stderr, "hint: did you mean %q?\n", s)
	}
	fmt.Fprint(in.stderr, "usage: truss <command> [arguments]\nRun \"truss help\" to see every command.\n")
	return 1
}

// commandUsage explains wrong arguments to a known command: the command's own
// usage, not the whole list. It returns false when the command has no topic.
func (in invocation) commandUsage(command string) bool {
	key := command
	if a, ok := helpAliases[key]; ok {
		key = a
	}
	topic, ok := helpTopics[key]
	if !ok {
		return false
	}
	msg := fmt.Sprintf("wrong arguments for truss %s", command)
	if len(in.args) > 0 {
		if subs := subcommands[command]; subs != nil && !strings.HasPrefix(in.args[0], "-") && !contains(subs, in.args[0]) {
			if s := suggest(in.args[0], subs); s != "" {
				msg = fmt.Sprintf("truss %s has no %q; did you mean %q?", command, in.args[0], s)
			}
		}
	}
	synopsis, _, _ := strings.Cut(topic, "\n\n")
	fmt.Fprintf(in.stderr, "error: %s\nusage: %s\nRun \"truss help %s\" for details.\n", msg, strings.ReplaceAll(synopsis, "\n", "\n       "), key)
	return true
}

// splitHint moves a trailing "(... with: ...)" advice out of an error message
// so it can be printed on a line of its own.
func splitHint(msg string) (text, hint string) {
	if !strings.HasSuffix(msg, ")") {
		return msg, ""
	}
	open := strings.LastIndex(msg, " (")
	if open < 0 {
		return msg, ""
	}
	inner := msg[open+2 : len(msg)-1]
	if !strings.Contains(inner, " with: ") || strings.ContainsAny(inner, "()") {
		return msg, ""
	}
	return msg[:open], inner
}

// flowArgs are the commands whose next word may be a flow name; runs also
// takes the subcommand show there.
var flowArgs = []string{"run", "validate", "register", "unregister", "pause", "resume", "trigger", "runs", "cache"}

// completion prints a completion script for a shell. Flow names are the
// .yaml and .yml files of .truss/flows/ in the current folder, the project
// TRUSS uses (spec 025).
func (in invocation) completion() int {
	if len(in.args) != 1 {
		return in.usage()
	}
	words := strings.Join(commands, " ")
	keys := make([]string, 0, len(subcommands))
	for k := range subcommands {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	subs := func(k string) string { return strings.Join(subcommands[k], " ") }
	var b strings.Builder
	switch in.args[0] {
	case "bash":
		b.WriteString("# truss completion for bash. Load with: eval \"$(truss completion bash)\"\n")
		b.WriteString("_truss_flows() {\n  local f\n  for f in .truss/flows/*.yaml .truss/flows/*.yml; do\n")
		b.WriteString("    [ -f \"$f\" ] || continue\n    f=${f##*/}\n    printf '%s\\n' \"${f%.*}\"\n  done\n}\n")
		b.WriteString("_truss() {\n  local cur=${COMP_WORDS[COMP_CWORD]} cmd=${COMP_WORDS[1]} IFS=$'\\n'\n")
		b.WriteString("  if [ \"$COMP_CWORD\" -eq 1 ]; then COMPREPLY=($(compgen -W \"" + strings.ReplaceAll(words, " ", "\n") + "\" -- \"$cur\")); return; fi\n")
		b.WriteString("  if [ \"$COMP_CWORD\" -eq 2 ]; then case \"$cmd\" in\n")
		b.WriteString("    runs) COMPREPLY=($(compgen -W \"show\n$(_truss_flows)\" -- \"$cur\")); return;;\n")
		for _, k := range keys {
			if k == "runs" {
				continue
			}
			fmt.Fprintf(&b, "    %s) COMPREPLY=($(compgen -W \"%s\" -- \"$cur\")); return;;\n", k, strings.ReplaceAll(subs(k), " ", "\n"))
		}
		b.WriteString("  esac; fi\n")
		b.WriteString("  COMPREPLY=($(compgen -W \"$(_truss_flows)\" -- \"$cur\"))\n}\ncomplete -o default -F _truss truss\n")
	case "zsh":
		b.WriteString("#compdef truss\n# truss completion for zsh. Load with: eval \"$(truss completion zsh)\"\n")
		b.WriteString("(( $+functions[compdef] )) || { autoload -Uz compinit && compinit }\n")
		b.WriteString("_truss() {\n  local -a flows\n  flows=(.truss/flows/*.(yaml|yml)(N:t:r))\n")
		b.WriteString("  if (( CURRENT == 2 )); then compadd -- " + words + "; return; fi\n")
		b.WriteString("  if (( CURRENT == 3 )); then case $words[2] in\n")
		b.WriteString("    runs) compadd -- show $flows; return;;\n")
		for _, k := range keys {
			if k == "runs" {
				continue
			}
			fmt.Fprintf(&b, "    %s) compadd -- %s; return;;\n", k, subs(k))
		}
		b.WriteString("  esac; fi\n  compadd -- $flows\n  _files\n}\ncompdef _truss truss\n")
	case "fish":
		b.WriteString("# truss completion for fish. Load with: truss completion fish | source\n")
		b.WriteString("function __truss_flows\n  for f in .truss/flows/*.yaml .truss/flows/*.yml\n")
		b.WriteString("    string replace -r '\\.ya?ml$' '' -- (basename $f)\n  end\nend\n")
		b.WriteString("complete -c truss -f -n '__fish_use_subcommand' -a '" + words + "'\n")
		for _, k := range keys {
			fmt.Fprintf(&b, "complete -c truss -f -n '__fish_seen_subcommand_from %s; and test (count (commandline -opc)) -eq 2' -a '%s'\n", k, subs(k))
		}
		b.WriteString("complete -c truss -f -n '__fish_seen_subcommand_from " + strings.Join(flowArgs, " ") + "; and test (count (commandline -opc)) -eq 2' -a '(__truss_flows)'\n")
		b.WriteString("complete -c truss -f -n '__fish_seen_subcommand_from flows pipelines; and test (count (commandline -opc)) -eq 3' -a '(__truss_flows)'\n")
	case "powershell":
		b.WriteString("# truss completion for PowerShell. Load with: truss completion powershell | Out-String | Invoke-Expression\n")
		b.WriteString("Register-ArgumentCompleter -Native -CommandName 'truss', 'truss.exe' -ScriptBlock {\n  param($word, $ast, $cursor)\n")
		b.WriteString("  $parts = @($ast.CommandElements | ForEach-Object { $_.ToString() })\n")
		b.WriteString("  $commands = '" + words + "' -split ' '\n  $subs = @{\n")
		for _, k := range keys {
			fmt.Fprintf(&b, "    '%s' = '%s' -split ' '\n", k, subs(k))
		}
		b.WriteString("  }\n")
		b.WriteString("  $flows = @(Get-ChildItem -Path .truss/flows -File -ErrorAction SilentlyContinue | Where-Object { $_.Extension -in '.yaml', '.yml' } | ForEach-Object { $_.BaseName })\n")
		b.WriteString("  $n = $parts.Count; if ($word -ne '') { $n -= 1 }\n")
		b.WriteString("  if ($n -le 1) { $list = $commands }\n")
		b.WriteString("  elseif ($n -eq 2 -and $parts[1] -eq 'runs') { $list = @('show') + $flows }\n")
		b.WriteString("  elseif ($n -eq 2 -and $subs.ContainsKey($parts[1])) { $list = $subs[$parts[1]] }\n")
		b.WriteString("  else { $list = $flows }\n")
		b.WriteString("  $list | Where-Object { $_.StartsWith($word, [System.StringComparison]::OrdinalIgnoreCase) } | ForEach-Object { [System.Management.Automation.CompletionResult]::new($_) }\n}\n")
	default:
		return in.usage()
	}
	fmt.Fprint(in.stdout, b.String())
	return 0
}
