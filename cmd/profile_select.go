package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode"

	survey "github.com/AlecAivazis/survey/v2"
	"golang.org/x/term"
)

type profileOption struct {
	Dir      string
	Friendly string
	Path     string
	Exists   bool
}

const createProfileOptionLabel = "Create new profile…"

func ensureDesktopProfileSelection(logf func(string, ...interface{})) error {
	if desktopProfileSelectionDone {
		return nil
	}
	desktopProfileSelectionDone = true

	defaultDir := profileDefaultFlag
	if defaultDir == "" {
		defaultDir = defaultDesktopProfileDir
	}

	userDataRootWin, err := resolveWindowsUserDataRoot(logf)
	if err != nil {
		return err
	}

	localStatePath, err := resolveLocalStatePath(userDataRootWin)
	if err != nil {
		return err
	}
	friendlyNames, err := readChromeProfileNames(localStatePath)
	if err != nil {
		logf("warning: unable to read profile names from %s: %v\n", localStatePath, err)
		friendlyNames = map[string]string{}
	}

	fsRoot, err := windowsToHostPath(userDataRootWin)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(fsRoot, 0755); err != nil {
		return err
	}

	options, err := discoverProfileOptions(fsRoot, friendlyNames)
	if err != nil {
		return err
	}
	if !hasProfileDir(options, defaultDir) {
		options = append(options, profileOption{
			Dir:      defaultDir,
			Friendly: friendlyNames[defaultDir],
			Path:     filepath.Join(fsRoot, defaultDir),
			Exists:   false,
		})
	}

	var (
		selected profileOption
		created  bool
	)

	if profileNameFlag != "" {
		selected = chooseOptionByDir(options, profileNameFlag)
		if selected.Dir == "" {
			selected = profileOption{
				Dir:      profileNameFlag,
				Friendly: friendlyNames[profileNameFlag],
				Path:     filepath.Join(fsRoot, profileNameFlag),
				Exists:   false,
			}
		}
	} else {
		shouldPrompt := term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
		if profilePrompt && !shouldPrompt {
			logf("warning: --profile-prompt requested but no interactive terminal available; falling back to default\n")
		}
		if (profilePrompt || profileNameFlag == "") && shouldPrompt {
			selected, created, err = promptForProfile(options, fsRoot, defaultDir, friendlyNames)
			if err != nil {
				return err
			}
		} else {
			selected = chooseOptionByDir(options, defaultDir)
			if selected.Dir == "" {
				selected = profileOption{
					Dir:      defaultDir,
					Friendly: friendlyNames[defaultDir],
					Path:     filepath.Join(fsRoot, defaultDir),
					Exists:   false,
				}
			}
		}
	}

	if selected.Dir == "" {
		selected = profileOption{
			Dir:      defaultDesktopProfileDir,
			Path:     filepath.Join(fsRoot, defaultDesktopProfileDir),
			Friendly: friendlyNames[defaultDesktopProfileDir],
		}
	}

	if err := ensureProfileDirectory(selected.Path); err != nil {
		return err
	}

	resolvedDesktopProfileDir = selected.Dir

	// Determine which title to apply.
	if profileTitleFlag != "" {
		resolvedDesktopProfileTitle = profileTitleFlag
		applyDesktopProfileTitle = true
	} else if created {
		resolvedDesktopProfileTitle = firstNonEmpty(selected.Friendly, selected.Dir)
		applyDesktopProfileTitle = resolvedDesktopProfileTitle != ""
	} else if selected.Friendly != "" {
		resolvedDesktopProfileTitle = selected.Friendly
		applyDesktopProfileTitle = true
	} else {
		resolvedDesktopProfileTitle = ""
		applyDesktopProfileTitle = false
	}

	logf("desktop profile selected: dir=%s title=%q\n", resolvedDesktopProfileDir, resolvedDesktopProfileTitle)
	fmt.Fprintf(os.Stderr, "[profile] using dir=%s title=%q\n", resolvedDesktopProfileDir, resolvedDesktopProfileTitle)
	if created {
		fmt.Fprintf(os.Stderr, "[profile] created new profile dir=%s at %s\n", resolvedDesktopProfileDir, filepath.Join(fsRoot, resolvedDesktopProfileDir))
	}

	return nil
}

func resolveWindowsUserDataRoot(logf func(string, ...interface{})) (string, error) {
	if runtime.GOOS == "windows" {
		winProfile := os.Getenv("USERPROFILE")
		if winProfile == "" {
			return "", fmt.Errorf("USERPROFILE not set")
		}
		return filepath.Join(winProfile, "AppData", "Local", "Google", "Chrome", "User Data"), nil
	}

	profCmd := exec.Command("cmd.exe", "/C", "echo", "%USERPROFILE%")
	out, err := profCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to resolve %%USERPROFILE%%: %w", err)
	}
	winProfile := strings.TrimSpace(string(out))
	if winProfile == "" {
		logf("warning: USERPROFILE expanded to empty string, using default data-dir\n")
		return `C:\Users\WSL\AppData\Local\Google\Chrome\User Data`, nil
	}
	return winProfile + `\AppData\Local\Google\Chrome\User Data`, nil
}

func ensureDesktopProfileTitle(logf func(string, ...interface{}), userDataRootWin, profileDir, title string) error {
	if title == "" {
		return nil
	}

	localStatePath, err := resolveLocalStatePath(userDataRootWin)
	if err != nil {
		return fmt.Errorf("resolve Local State path: %w", err)
	}
	if err := updateChromeLocalState(localStatePath, profileDir, title); err != nil {
		return fmt.Errorf("update Local State: %w", err)
	}
	logf("set Chrome profile %s title to %q via %s\n", profileDir, title, localStatePath)
	return nil
}

func resolveLocalStatePath(userDataRootWin string) (string, error) {
	localStateWin := userDataRootWin + `\Local State`
	return windowsToHostPath(localStateWin)
}

func windowsToHostPath(winPath string) (string, error) {
	if runtime.GOOS == "windows" {
		return winPath, nil
	}
	cmd := exec.Command("wslpath", "-u", winPath)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("wslpath conversion failed for %s: %w", winPath, err)
	}
	linuxPath := strings.TrimSpace(string(out))
	if linuxPath == "" {
		return "", fmt.Errorf("empty path after wslpath conversion for %s", winPath)
	}
	return linuxPath, nil
}

func updateChromeLocalState(localStatePath, profileDir, title string) error {
	var data map[string]interface{}

	raw, err := os.ReadFile(localStatePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		data = make(map[string]interface{})
	} else {
		if len(bytes.TrimSpace(raw)) == 0 {
			data = make(map[string]interface{})
		} else if err := json.Unmarshal(raw, &data); err != nil {
			return fmt.Errorf("parse Local State: %w", err)
		}
	}
	if data == nil {
		data = make(map[string]interface{})
	}

	profileObj, _ := data["profile"].(map[string]interface{})
	if profileObj == nil {
		profileObj = make(map[string]interface{})
	}
	infoCache, _ := profileObj["info_cache"].(map[string]interface{})
	if infoCache == nil {
		infoCache = make(map[string]interface{})
	}
	entry, _ := infoCache[profileDir].(map[string]interface{})
	if entry == nil {
		entry = make(map[string]interface{})
	}

	entry["name"] = title
	entry["is_using_default_name"] = false
	infoCache[profileDir] = entry
	profileObj["info_cache"] = infoCache
	data["profile"] = profileObj

	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(localStatePath), 0755); err != nil {
		return err
	}

	mode := os.FileMode(0644)
	if fi, err := os.Stat(localStatePath); err == nil {
		mode = fi.Mode()
	}

	tmpPath := localStatePath + ".tmp"
	if err := os.WriteFile(tmpPath, encoded, mode); err != nil {
		return err
	}
	return os.Rename(tmpPath, localStatePath)
}

func readChromeProfileNames(localStatePath string) (map[string]string, error) {
	raw, err := os.ReadFile(localStatePath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return map[string]string{}, nil
	}
	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}

	profileObj, _ := data["profile"].(map[string]interface{})
	if profileObj == nil {
		return map[string]string{}, nil
	}
	infoCache, _ := profileObj["info_cache"].(map[string]interface{})
	if infoCache == nil {
		return map[string]string{}, nil
	}

	result := make(map[string]string)
	for dir, rawEntry := range infoCache {
		if entry, ok := rawEntry.(map[string]interface{}); ok {
			if name, ok := entry["name"].(string); ok && name != "" {
				result[dir] = name
			}
		}
	}
	return result, nil
}

func discoverProfileOptions(fsRoot string, friendly map[string]string) ([]profileOption, error) {
	var options []profileOption
	entries, err := os.ReadDir(fsRoot)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			options = append(options, profileOption{
				Dir:      name,
				Friendly: friendly[name],
				Path:     filepath.Join(fsRoot, name),
				Exists:   true,
			})
		}
	}

	for dir, friendlyName := range friendly {
		if hasProfileDir(options, dir) {
			continue
		}
		options = append(options, profileOption{
			Dir:      dir,
			Friendly: friendlyName,
			Path:     filepath.Join(fsRoot, dir),
			Exists:   false,
		})
	}

	sort.SliceStable(options, func(i, j int) bool {
		li := strings.ToLower(profileSortKey(options[i]))
		lj := strings.ToLower(profileSortKey(options[j]))
		if li == lj {
			return strings.ToLower(options[i].Dir) < strings.ToLower(options[j].Dir)
		}
		return li < lj
	})

	return options, nil
}

func profileSortKey(opt profileOption) string {
	if opt.Friendly != "" {
		return opt.Friendly
	}
	return opt.Dir
}

func hasProfileDir(options []profileOption, dir string) bool {
	for _, opt := range options {
		if strings.EqualFold(opt.Dir, dir) {
			return true
		}
	}
	return false
}

func chooseOptionByDir(options []profileOption, dir string) profileOption {
	for _, opt := range options {
		if strings.EqualFold(opt.Dir, dir) {
			return opt
		}
	}
	return profileOption{}
}

func promptForProfile(options []profileOption, fsRoot, defaultDir string, friendly map[string]string) (profileOption, bool, error) {
	labels := make([]string, 0, len(options)+1)
	labelToOption := make(map[string]profileOption, len(options))
	defaultLabel := ""

	for _, opt := range options {
		label := formatProfileLabel(opt)
		if strings.EqualFold(opt.Dir, defaultDir) {
			defaultLabel = label
		}
		labels = append(labels, label)
		labelToOption[label] = opt
	}
	labels = append(labels, createProfileOptionLabel)
	if defaultLabel == "" && len(options) > 0 {
		defaultLabel = formatProfileLabel(options[0])
	}

	var selection string
	prompt := &survey.Select{
		Message: "Select Chrome profile",
		Options: labels,
		Default: defaultLabel,
	}
	if err := survey.AskOne(prompt, &selection); err != nil {
		return profileOption{}, false, err
	}

	if selection == createProfileOptionLabel {
		opt, err := promptCreateProfile(fsRoot, friendly, labelToOption)
		return opt, true, err
	}

	opt, ok := labelToOption[selection]
	if !ok {
		return profileOption{}, false, fmt.Errorf("unknown profile selection: %s", selection)
	}
	return opt, false, nil
}

func formatProfileLabel(opt profileOption) string {
	name := opt.Friendly
	if name == "" {
		name = opt.Dir
	}
	if strings.EqualFold(name, opt.Dir) {
		return name
	}
	return fmt.Sprintf("%s (%s)", name, opt.Dir)
}

func promptCreateProfile(fsRoot string, friendly map[string]string, existing map[string]profileOption) (profileOption, error) {
	var displayName string
	if err := survey.AskOne(&survey.Input{
		Message: "Profile display name",
	}, &displayName, survey.WithValidator(survey.Required)); err != nil {
		return profileOption{}, err
	}
	displayName = strings.TrimSpace(displayName)

	slug := slugify(displayName)
	if slug == "" {
		slug = "Profile"
	}

	used := make(map[string]bool, len(existing))
	for _, opt := range existing {
		used[strings.ToLower(opt.Dir)] = true
	}

	dir := ensureUniqueDir(slug, used)

	if err := survey.AskOne(&survey.Input{
		Message: "Profile directory",
		Default: dir,
	}, &dir, survey.WithValidator(validateProfileDir)); err != nil {
		return profileOption{}, err
	}
	dir = strings.TrimSpace(dir)
	dirSlug := slugify(dir)
	if dirSlug == "" {
		return profileOption{}, fmt.Errorf("invalid profile directory")
	}
	if used[strings.ToLower(dirSlug)] {
		dirSlug = ensureUniqueDir(dirSlug, used)
	}

	path := filepath.Join(fsRoot, dirSlug)
	if err := os.MkdirAll(path, 0755); err != nil {
		return profileOption{}, err
	}

	used[strings.ToLower(dirSlug)] = true
	friendly[dirSlug] = displayName
	return profileOption{
		Dir:      dirSlug,
		Friendly: displayName,
		Path:     path,
		Exists:   true,
	}, nil
}

func ensureProfileDirectory(path string) error {
	return os.MkdirAll(path, 0755)
}

func slugify(input string) string {
	var b strings.Builder
	lastHyphen := false
	for _, r := range strings.ToLower(input) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastHyphen = false
		case r == '-' || r == '_' || unicode.IsSpace(r):
			if !lastHyphen {
				b.WriteRune('-')
				lastHyphen = true
			}
		}
	}
	result := strings.Trim(b.String(), "-")
	return result
}

func ensureUniqueDir(base string, used map[string]bool) string {
	candidate := base
	index := 1
	for used[strings.ToLower(candidate)] {
		index++
		candidate = fmt.Sprintf("%s-%d", base, index)
	}
	return candidate
}

func validateProfileDir(ans interface{}) error {
	str, _ := ans.(string)
	str = strings.TrimSpace(str)
	if str == "" {
		return fmt.Errorf("directory name is required")
	}
	for _, r := range str {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_') {
			return fmt.Errorf("directory may only contain letters, numbers, '-' or '_'")
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
