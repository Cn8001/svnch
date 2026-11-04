package svn

// --parents adds the unversioned folders above a file too, so picking a file deep in a new folder just works.
func AddArgs(paths ...string) ([]string, error) {
	t, err := targets(paths...)
	if err != nil {
		return nil, err
	}
	return append([]string{"--parents"}, t...), nil
}

func DeleteArgs(keepLocal bool, paths ...string) ([]string, error) {
	t, err := targets(paths...)
	if err != nil {
		return nil, err
	}
	if keepLocal {
		return append([]string{"--keep-local"}, t...), nil
	}
	// --force because svn refuses to delete a file with local edits, and the person has already confirmed that
	// the files go from disk.
	return append([]string{"--force"}, t...), nil
}

// Revert goes to infinity depth because svn refuses a plain revert of an added folder.
func RevertArgs(paths ...string) ([]string, error) {
	t, err := targets(paths...)
	if err != nil {
		return nil, err
	}
	return append([]string{"--depth", "infinity"}, t...), nil
}

func MoveArgs(from, to string) ([]string, error) {
	return targets(from, to)
}
