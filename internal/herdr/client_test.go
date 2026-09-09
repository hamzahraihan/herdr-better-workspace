package herdr

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestValidateName(t *testing.T) {
	if err := ValidateName("demo"); err != nil {
		t.Fatalf("valid name rejected: %v", err)
	}
	for _, bad := range []string{"", "   ", strings.Repeat("x", 65), "a\nb", "a\tb"} {
		if err := ValidateName(bad); err == nil {
			t.Fatalf("invalid name %q accepted", bad)
		}
	}
}

func TestValidatePath(t *testing.T) {
	if err := ValidatePath("~/Projects/demo"); err != nil {
		t.Fatalf("valid path rejected: %v", err)
	}
	for _, bad := range []string{"", "   ", "a\x00b"} {
		if err := ValidatePath(bad); err == nil {
			t.Fatalf("invalid path %q accepted", bad)
		}
	}
}

func TestResolvePathTilde(t *testing.T) {
	got, err := ResolvePath("~/x")
	if err != nil {
		t.Fatalf("ResolvePath: %v", err)
	}
	if !strings.HasSuffix(got, "x") || strings.Contains(got, "~") {
		t.Fatalf("tilde not expanded: %q", got)
	}
}

func TestArgsShape(t *testing.T) {
	o := CreateOptions{Name: "demo", TemplateID: "go", Focus: true}
	args := o.Args(`C:\work\demo`)
	want := []string{"workspace", "create", "--label", "demo", "--cwd", `C:\work\demo`, "--focus"}
	if strings.Join(args, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("args = %q, want %q", args, want)
	}
	o.Focus = false
	args = o.Args(`C:\work\demo`)
	if args[len(args)-1] != "--no-focus" {
		t.Fatalf("unfocused args missing --no-focus: %q", args)
	}
}

func TestValidateRejectsUnknownTemplate(t *testing.T) {
	o := CreateOptions{Name: "demo", Path: "/tmp/demo", TemplateID: "nope"}
	if err := Validate(o); err == nil {
		t.Fatal("unknown template accepted")
	}
}

func TestCreateSkipScaffoldLeavesDirUntouched(t *testing.T) {
	dir := t.TempDir()
	target := dir + "/existing"
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := target + "/keep.txt"
	if err := os.WriteFile(sentinel, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Create(context.Background(), CreateOptions{
		Name: "existing", Path: target, TemplateID: "blank",
		SkipScaffold: true, InitGit: false, DryRun: true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if res.Path == "" {
		t.Fatal("empty result path")
	}
	ents, err := os.ReadDir(target)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 1 || ents[0].Name() != "keep.txt" {
		names := []string{}
		for _, e := range ents {
			names = append(names, e.Name())
		}
		t.Fatalf("directory touched: %q", names)
	}
}
