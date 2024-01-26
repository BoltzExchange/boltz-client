package build

var Commit string

const version = "1.3.0-rc1"

func GetVersion() string {
	basicVersion := "v" + version

	if Commit == "" {
		return basicVersion
	}

	return basicVersion + "-" + Commit
}
