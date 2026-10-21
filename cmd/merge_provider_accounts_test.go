package cmd

import (
	"bigbucks/solution/auth/models"
	"bytes"
	"testing"
)

func TestPrintOutcome(t *testing.T) {
	var out bytes.Buffer
	printOutcome(&out, "merge", models.MergeOutcome{
		Address:   "ana@acme.example",
		KeptID:    "01KEPT",
		Renamed:   "ana@acme.example+google",
		Merged:    []string{"ana@acme.example+microsoft"},
		Notes:     []string{"organization 01ORG: role Owner from ana@acme.example+microsoft replaces ana@acme.example's"},
		Conflicts: []string{"other: deactivated"},
	})

	want := "rename   ana@acme.example+google -> ana@acme.example (01KEPT)\n" +
		"merge    ana@acme.example+microsoft -> ana@acme.example (01KEPT)\n" +
		"  note   organization 01ORG: role Owner from ana@acme.example+microsoft replaces ana@acme.example's\n" +
		"CONFLICT other: deactivated\n"
	if out.String() != want {
		t.Fatalf("printOutcome() wrote\n%s\nwant\n%s", out.String(), want)
	}
}

func TestPrintOutcomeNothingToDo(t *testing.T) {
	var out bytes.Buffer
	printOutcome(&out, "dry-run", models.MergeOutcome{Address: "ana@acme.example"})
	if out.Len() != 0 {
		t.Fatalf("printOutcome() of an empty outcome wrote %q", out.String())
	}
}
