package misc

import (
	"fmt"
	"os"
	"testing"
)

func TestTriggerEmergencyLockDown(t *testing.T) {
	if StartupTime.Year() < 2016 {
		t.Fatal("start time is wrong")
	}
	TriggerEmergencyLockDown()
	if !EmergencyLockDown {
		t.Fatal("did not trigger")
	}
}

func TestOverwriteWithZero(t *testing.T) {
	fh, err := os.CreateTemp("", "laitos-TestOverwriteWithZero")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(fh.Name())
	defer fh.Close()
	if _, err := fh.WriteString("abcde"); err != nil {
		t.Fatal(err)
	}
	if err := fh.Close(); err != nil {
		t.Fatal(err)
	}
	if err := overwriteWithZero(fh.Name()); err != nil {
		t.Fatal(err)
	}
	// Reopen and verify that content is empty
	reopened, err := os.Open(fh.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	buf := make([]byte, 1000)
	n, err := reopened.Read(buf)
	if err != nil || n != 5 {
		t.Fatal(err, n)
	}
}

func TestGetDirsToKill(t *testing.T) {
	toKill := getDirsToKill()
	fmt.Println(toKill)
	// At least it should kill /, program directory, and parent to program directory.
	if len(toKill) < 3 {
		t.Fatal(toKill)
	}
}

func TestGetFilesToKill(t *testing.T) {
	toKill := getFilesToKill()
	fmt.Println(toKill)
	// At least it should kill the program executable itself
	if len(toKill) < 1 {
		t.Fatal(toKill)
	}
}
