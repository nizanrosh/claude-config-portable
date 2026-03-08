// Package skills collects user-created skill directories from ~/.claude/skills/,
// handling both regular directories and symlinks.
package skills

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nizanrosh/claude-config-portable/internal/payload"
)

// Collect walks the skills directory and returns all skill entries.
// Symlinks are recorded as references; real directories have their files read.
func Collect(skillsDir string) ([]payload.SkillEntry, error) {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading skills directory: %w", err)
	}

	var skills []payload.SkillEntry
	for _, entry := range entries {
		if !entry.IsDir() && entry.Type()&os.ModeSymlink == 0 {
			continue
		}

		fullPath := filepath.Join(skillsDir, entry.Name())
		skill, err := collectSkill(fullPath, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("collecting skill %q: %w", entry.Name(), err)
		}
		skills = append(skills, skill)
	}

	return skills, nil
}

func collectSkill(path, name string) (payload.SkillEntry, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return payload.SkillEntry{}, fmt.Errorf("stat %q: %w", path, err)
	}

	if fi.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return payload.SkillEntry{}, fmt.Errorf("reading symlink %q: %w", path, err)
		}
		return payload.SkillEntry{
			Name:       name,
			IsSymlink:  true,
			LinkTarget: target,
		}, nil
	}

	files, err := readDirFiles(path)
	if err != nil {
		return payload.SkillEntry{}, err
	}

	return payload.SkillEntry{
		Name:  name,
		Files: files,
	}, nil
}

// readDirFiles reads all files in a directory (non-recursive) into a map.
func readDirFiles(dir string) (map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %q: %w", dir, err)
	}

	files := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fullPath := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("reading file %q: %w", fullPath, err)
		}
		files[entry.Name()] = string(data)
	}
	return files, nil
}
