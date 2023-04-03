package build

var Commit string

const version = "1.2.7"

func GetVersion() string {
	basicVersion := "v" + version

	if Commit == "" {
		return basicVersion
	}

	return basicVersion + "-" + Commit
}
