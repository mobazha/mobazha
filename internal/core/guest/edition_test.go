package guest

import (
	"fmt"
	"os"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/edition"
)

func TestMain(m *testing.M) {
	if err := edition.ConfigureCurrentPolicy(edition.FullName); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	os.Exit(m.Run())
}
