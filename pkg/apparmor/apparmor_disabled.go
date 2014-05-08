// +build !apparmor !linux !amd64,!386

package apparmor

func IsEnabled() bool {
	return false
}

func ApplyProfile(name string) error {
	return nil
}
