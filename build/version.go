package build

var Commit string
var Version string

func GetVersion() string {
	if Version == "" {
		return Commit
	}
	basicVersion := "v" + Version

	if Commit == "" {
		return basicVersion
	}

	return basicVersion + "-" + Commit
}
