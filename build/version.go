package build

var Commit string

const version = "1.1.2"

func GetVersion() string {
	return "v" + version + "-" + Commit
}
