//go:build e2e
// +build e2e

package e2e

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	exit := m.Run()
	os.Exit(exit)
}

func TestNoop(t *testing.T) {

}
