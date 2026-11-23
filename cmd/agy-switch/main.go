// Command agy-switch switches between Antigravity CLI (agy) accounts.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

const usage = `usage: agy-switch [--version] COMMAND [ARGS]

Switch between Antigravity CLI (agy) accounts.

commands:
  add NAME [--login] [-f|--force]  save the account currently logged in to agy as a profile
                                   --login: log out, open agy to sign in to another
                                   account, and save the result
                                   --force: overwrite an existing profile
  list, ls                         list saved profiles
  use, switch NAME                 switch to the given profile
  current                          show the active profile
  remove, rm NAME [-y|--yes]       remove a saved profile (-y: do not ask for confirmation)
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(argv []string) int {
	if len(argv) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
	cmd, rest := argv[0], argv[1:]
	var err error
	code := 0
	switch cmd {
	case "-h", "--help", "help":
		fmt.Print(usage)
		return 0
	case "--version", "-V":
		fmt.Printf("agy-switch %s\n", version)
		return 0
	case "add":
		code, err = cmdAdd(rest)
	case "list", "ls":
		code, err = cmdList(rest)
	case "use", "switch":
		code, err = cmdUse(rest)
	case "current":
		code, err = cmdCurrent(rest)
	case "remove", "rm":
		code, err = cmdRemove(rest)
	default:
		fmt.Fprintf(os.Stderr, "agy-switch: unknown command %q\n\n%s", cmd, usage)
		return 2
	}
	var ue usageError
	if errors.As(err, &ue) {
		fmt.Fprintf(os.Stderr, "agy-switch %s: %s\n", cmd, ue.msg)
		return 2
	}
	if errors.Is(err, errHelp) {
		fmt.Print(usage)
		return 0
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		return 1
	}
	return code
}

// ---------------------------------------------------------------- argument parsing

type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

var errHelp = errors.New("help requested")

type parsedArgs struct {
	pos   []string
	bools map[string]bool
	vals  map[string]string
}

// parseArgs accepts flags and positionals in any order. bools and vals map every
// accepted spelling (e.g. "-f", "--force") to a canonical name.
func parseArgs(args []string, nPos int, bools, vals map[string]string) (*parsedArgs, error) {
	p := &parsedArgs{bools: map[string]bool{}, vals: map[string]string{}}
	onlyPos := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if onlyPos || a == "-" || !strings.HasPrefix(a, "-") {
			p.pos = append(p.pos, a)
			continue
		}
		if a == "--" {
			onlyPos = true
			continue
		}
		if a == "-h" || a == "--help" {
			return nil, errHelp
		}
		name, value, hasValue := strings.Cut(a, "=")
		if canon, ok := bools[name]; ok && !hasValue {
			p.bools[canon] = true
			continue
		}
		if canon, ok := vals[name]; ok {
			if !hasValue {
				if i+1 >= len(args) {
					return nil, usageError{fmt.Sprintf("option %s needs a value", name)}
				}
				i++
				value = args[i]
			}
			p.vals[canon] = value
			continue
		}
		return nil, usageError{fmt.Sprintf("unknown option %s", a)}
	}
	if len(p.pos) != nPos {
		if nPos == 0 {
			return nil, usageError{"takes no arguments"}
		}
		return nil, usageError{"expected NAME"}
	}
	return p, nil
}

// ---------------------------------------------------------------- commands

func cmdAdd(args []string) (int, error) {
	p, err := parseArgs(args, 1,
		map[string]string{"--login": "login", "-f": "force", "--force": "force"},
		map[string]string{"--agy-bin": "agy-bin"})
	if err != nil {
		return 0, err
	}
	name, err := validateName(p.pos[0])
	if err != nil {
		return 0, err
	}
	exists := isFile(tokenPath(name))
	if exists && !p.bools["force"] {
		return 0, fmt.Errorf("profile %q already exists (use --force to overwrite)", name)
	}

	if p.bools["login"] {
		agyBin := p.vals["agy-bin"]
		if agyBin == "" {
			agyBin = os.Getenv("AGY_BIN")
		}
		if agyBin == "" {
			agyBin = "agy"
		}
		if err := loginNewAccount(agyBin); err != nil {
			return 0, err
		}
	}

	email := liveEmail()
	if email != "" {
		for _, other := range listProfiles() {
			if other != name && profileEmail(other) == email {
				fmt.Fprintf(os.Stderr, "warning: account %s is already saved in profile %q\n", email, other)
			}
		}
	}
	if err := snapshotLive(name); err != nil {
		return 0, err
	}
	st := loadState()
	st.Active = &name
	if err := saveState(st); err != nil {
		return 0, err
	}
	verb := "added"
	if exists {
		verb = "updated"
	}
	fmt.Printf("Profile %q %s (%s) and set as active.\n", name, verb, orDefault(email, "unknown email"))
	return 0, nil
}

func loginNewAccount(agyBin string) error {
	if err := syncActiveBack(); err != nil {
		return err
	}
	backup, err := backupLive()
	if err != nil {
		return err
	}
	restore := func() {
		if backup != "" {
			if data, err := os.ReadFile(backup); err == nil {
				_ = writeLiveToken(data)
			}
		}
	}
	if err := clearLiveToken(); err != nil {
		return err
	}
	fmt.Println("Opening agy to log in. Sign in with the desired account and, once " +
		"logged in, exit agy (/exit or Ctrl+C).")

	bin, err := exec.LookPath(agyBin)
	if err != nil {
		restore()
		return fmt.Errorf("executable %q not found", agyBin)
	}
	// Ctrl+C is meant for agy; keep it from killing agy-switch.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	c := exec.Command(bin)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	_ = c.Run()
	signal.Stop(sig)

	data, err := readLiveToken()
	if err != nil {
		return err
	}
	if data == nil {
		restore()
		return errors.New("login not completed; previous credentials restored")
	}
	return nil
}

func cmdList(args []string) (int, error) {
	if _, err := parseArgs(args, 0, nil, nil); err != nil {
		return 0, err
	}
	names := listProfiles()
	if len(names) == 0 {
		fmt.Println("No saved profiles. Use `agy-switch add NAME`.")
		return 0, nil
	}
	active := activeProfile()
	width := 0
	for _, n := range names {
		width = max(width, len(n))
	}
	for _, n := range names {
		mark := " "
		if n == active {
			mark = "*"
		}
		fmt.Printf("%s %-*s  %s\n", mark, width, n, orDefault(profileEmail(n), "-"))
	}
	return 0, nil
}

func cmdUse(args []string) (int, error) {
	p, err := parseArgs(args, 1, nil, nil)
	if err != nil {
		return 0, err
	}
	name, err := validateName(p.pos[0])
	if err != nil {
		return 0, err
	}
	if !isFile(tokenPath(name)) {
		return 0, fmt.Errorf("profile %q does not exist (see `agy-switch list`)", name)
	}
	email := profileEmail(name)
	if name == activeProfile() && liveEmail() == email {
		fmt.Printf("Already using %q (%s).\n", name, email)
		return 0, nil
	}
	if err := syncActiveBack(); err != nil {
		return 0, err
	}
	if _, err := backupLive(); err != nil {
		return 0, err
	}
	if err := restoreProfile(name); err != nil {
		return 0, err
	}
	st := loadState()
	st.Active = &name
	if err := saveState(st); err != nil {
		return 0, err
	}
	fmt.Printf("Now using %q (%s).\n", name, orDefault(email, "unknown email"))
	return 0, nil
}

func cmdCurrent(args []string) (int, error) {
	if _, err := parseArgs(args, 0, nil, nil); err != nil {
		return 0, err
	}
	active := activeProfile()
	live := liveEmail()
	if active == "" {
		if live != "" {
			fmt.Printf("No active profile (agy is logged in as %s, not saved).\n", live)
		} else {
			fmt.Println("No active profile.")
		}
		return 1, nil
	}
	expected := profileEmail(active)
	fmt.Printf("%s (%s)\n", active, orDefault(expected, "unknown email"))
	if live != expected {
		fmt.Fprintf(os.Stderr, "warning: agy is logged in as %s, which differs from profile %q\n",
			orDefault(live, "nobody"), active)
	}
	return 0, nil
}

func cmdRemove(args []string) (int, error) {
	p, err := parseArgs(args, 1, map[string]string{"-y": "yes", "--yes": "yes"}, nil)
	if err != nil {
		return 0, err
	}
	name, err := validateName(p.pos[0])
	if err != nil {
		return 0, err
	}
	pdir := profileDir(name)
	if !isDir(pdir) {
		return 0, fmt.Errorf("profile %q does not exist", name)
	}
	if !p.bools["yes"] {
		fmt.Printf("Remove profile %q (%s)? [y/N] ", name, orDefault(profileEmail(name), "-"))
		answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Cancelled.")
			return 1, nil
		}
	}
	if err := os.RemoveAll(pdir); err != nil {
		return 0, err
	}
	st := loadState()
	if st.Active != nil && *st.Active == name {
		st.Active = nil
		if err := saveState(st); err != nil {
			return 0, err
		}
		fmt.Printf("Profile %q removed. It was the active one; the current agy "+
			"login was left unchanged.\n", name)
	} else {
		fmt.Printf("Profile %q removed.\n", name)
	}
	return 0, nil
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
