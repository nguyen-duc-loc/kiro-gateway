package kiro

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Source bytes are read only into runner memory, never diagnostic output.
func fixtureSource(repo, name string) ([]byte, error) {
	root, err := os.OpenRoot(repo)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	info, err := root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return nil, fmt.Errorf("fixture source unavailable")
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, fmt.Errorf("fixture source changed")
	}
	data, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		return nil, fmt.Errorf("fixture source unavailable")
	}
	return data, nil
}

func boundaryAssertion(repo string) (present, assertion bool, err error) {
	data, err := fixtureSource(repo, "clamp_test.go")
	if err != nil {
		return false, false, err
	}
	_, present, lines, err := boundaryAssertionLines(data)
	return present, len(lines) > 0, err
}

// Recognize a direct boundary comparison and its testing failure call. The
// isolated mutation below additionally proves this assertion actually executes.
func boundaryAssertionLines(data []byte) (*token.FileSet, bool, []int, error) {
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, "clamp_test.go", data, 0)
	if err != nil {
		return nil, false, nil, fmt.Errorf("fixture syntax invalid")
	}
	var boundary *ast.FuncDecl
	for _, decl := range file.Decls {
		f, ok := decl.(*ast.FuncDecl)
		if ok && f.Name.Name == "TestClampUpperBoundary" {
			if boundary != nil {
				return nil, true, nil, fmt.Errorf("fixture test ambiguous")
			}
			boundary = f
		}
	}
	if boundary == nil {
		return set, false, nil, nil
	}
	if boundary.Recv != nil || boundary.Body == nil || boundary.Type.Params == nil || len(boundary.Type.Params.List) != 1 {
		return set, true, nil, nil
	}
	param := boundary.Type.Params.List[0]
	pointer, ok := param.Type.(*ast.StarExpr)
	if !ok || len(param.Names) != 1 {
		return set, true, nil, nil
	}
	selector, ok := pointer.X.(*ast.SelectorExpr)
	if !ok || !ident(selector.X, "testing") || selector.Sel.Name != "T" {
		return set, true, nil, nil
	}
	receiver := param.Names[0].Name
	var lines []int
	ast.Inspect(boundary.Body, func(node ast.Node) bool {
		condition, ok := node.(*ast.IfStmt)
		if !ok {
			return true
		}
		comparison, ok := condition.Cond.(*ast.BinaryExpr)
		if !ok || comparison.Op != token.NEQ {
			return true
		}
		actual, expected := comparison.X, comparison.Y
		if integer(actual, 10) {
			actual, expected = expected, actual
		}
		if !integer(expected, 10) {
			return true
		}
		valid := boundaryCall(actual)
		if assignment, ok := condition.Init.(*ast.AssignStmt); ok && len(assignment.Lhs) == 1 && len(assignment.Rhs) == 1 && assignment.Tok == token.DEFINE {
			name, ok := assignment.Lhs[0].(*ast.Ident)
			valid = ok && ident(actual, name.Name) && boundaryCall(assignment.Rhs[0])
		}
		if !valid {
			return true
		}
		ast.Inspect(condition.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			method, ok := call.Fun.(*ast.SelectorExpr)
			if ok && ident(method.X, receiver) && (method.Sel.Name == "Fatal" || method.Sel.Name == "Fatalf" || method.Sel.Name == "Error" || method.Sel.Name == "Errorf") {
				lines = append(lines, set.Position(call.Pos()).Line)
			}
			return true
		})
		return true
	})
	return set, true, lines, nil
}
func ident(e ast.Expr, name string) bool { id, ok := e.(*ast.Ident); return ok && id.Name == name }
func integer(e ast.Expr, value int64) bool {
	literal, ok := e.(*ast.BasicLit)
	if !ok || literal.Kind != token.INT {
		return false
	}
	n, err := strconv.ParseInt(literal.Value, 0, 64)
	return err == nil && n == value
}
func boundaryCall(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	return ok && ident(call.Fun, "Clamp") && len(call.Args) == 3 && integer(call.Args[0], 11) && integer(call.Args[1], 0) && integer(call.Args[2], 10)
}

func upperMutation(source []byte) ([]byte, bool) {
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, "clamp.go", source, 0)
	if err != nil {
		return nil, false
	}
	var clamp *ast.FuncDecl
	for _, decl := range file.Decls {
		f, ok := decl.(*ast.FuncDecl)
		if ok && f.Recv == nil && f.Name.Name == "Clamp" {
			if clamp != nil {
				return nil, false
			}
			clamp = f
		}
	}
	if clamp == nil || clamp.Body == nil || clamp.Type.TypeParams != nil || clamp.Type.Results == nil || len(clamp.Type.Results.List) != 1 || !ident(clamp.Type.Results.List[0].Type, "int") || len(clamp.Type.Results.List[0].Names) != 0 {
		return nil, false
	}
	names := []string{}
	for _, field := range clamp.Type.Params.List {
		if !ident(field.Type, "int") {
			return nil, false
		}
		for _, name := range field.Names {
			names = append(names, name.Name)
		}
	}
	if strings.Join(names, ",") != "value,lower,upper" {
		return nil, false
	}
	var target *ast.Ident
	for _, statement := range clamp.Body.List {
		branch, ok := statement.(*ast.IfStmt)
		if !ok {
			continue
		}
		condition, ok := branch.Cond.(*ast.BinaryExpr)
		if !ok || condition.Op != token.GTR || !ident(condition.X, "value") || !ident(condition.Y, "upper") {
			continue
		}
		if target != nil || branch.Init != nil || branch.Else != nil || len(branch.Body.List) != 1 {
			return nil, false
		}
		ret, ok := branch.Body.List[0].(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 || !ident(ret.Results[0], "upper") {
			return nil, false
		}
		target = ret.Results[0].(*ast.Ident)
	}
	if target == nil {
		return nil, false
	}
	start, end := set.Position(target.Pos()).Offset, set.Position(target.End()).Offset
	mutant := append([]byte(nil), source[:start]...)
	mutant = append(mutant, []byte("lower")...)
	mutant = append(mutant, source[end:]...)
	return mutant, true
}

func fixtureTest(ctx context.Context, repo string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "rtk", append([]string{"proxy", "go", "test"}, args...)...)
	cmd.Dir = repo
	// The Go driver owns compiler and test children. Cancel the process group so
	// a blocked test cannot outlive the runner deadline or keep output pipes open.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = time.Second
	for _, key := range []string{"PATH", "HOME", "TMPDIR", "GOCACHE"} {
		if value, ok := os.LookupEnv(key); ok {
			cmd.Env = append(cmd.Env, key+"="+value)
		}
	}
	cmd.Env = append(cmd.Env, "GOTOOLCHAIN=local", "GOWORK=off", "GOPROXY=off", "GOSUMDB=off", "GOENV=off", "GOFLAGS=-mod=readonly")
	// Bound compiler and test output in memory. Never print it in run evidence.
	output := &boundedFixtureOutput{}
	cmd.Stdout, cmd.Stderr = output, output
	err := cmd.Run()
	if output.exceeded {
		return nil, fmt.Errorf("fixture output limit")
	}
	return output.Bytes(), err
}

type boundedFixtureOutput struct {
	bytes.Buffer
	exceeded bool
}

func (b *boundedFixtureOutput) Write(p []byte) (int, error) {
	if len(p) > (1<<20)-b.Len() {
		b.exceeded = true
		return 0, fmt.Errorf("fixture output limit")
	}
	return b.Buffer.Write(p)
}

func verifyBoundaryRegression(ctx context.Context, repo string) bool {
	files := map[string][]byte{}
	for _, name := range []string{"go.mod", "clamp.go", "clamp_test.go"} {
		source, err := fixtureSource(repo, name)
		if err != nil {
			return false
		}
		files[name] = source
	}
	_, present, lines, err := boundaryAssertionLines(files["clamp_test.go"])
	if err != nil || !present || len(lines) == 0 {
		return false
	}
	mutant, ok := upperMutation(files["clamp.go"])
	if !ok {
		return false
	}
	dir, err := os.MkdirTemp("", "kiro-boundary-")
	if err != nil {
		return false
	}
	defer os.RemoveAll(dir)
	for _, name := range []string{"final", "mutant"} {
		path := filepath.Join(dir, name)
		if os.Mkdir(path, 0700) != nil {
			return false
		}
		for filename, source := range files {
			if name == "mutant" && filename == "clamp.go" {
				source = mutant
			}
			if os.WriteFile(filepath.Join(path, filename), source, 0600) != nil {
				return false
			}
		}
	}
	final, mutated := filepath.Join(dir, "final"), filepath.Join(dir, "mutant")
	if _, err := fixtureTest(ctx, final, "-count=1", "-run", "^TestClampUpperBoundary$", "."); err != nil {
		return false
	}
	// A successful build with no selected tests separates an assertion failure
	// from syntax, compiler, package initialization and setup failures.
	if _, err := fixtureTest(ctx, mutated, "-count=1", "-run", "^$", "."); err != nil {
		return false
	}
	output, err := fixtureTest(ctx, mutated, "-count=1", "-run", "^TestClampUpperBoundary$", ".")
	if err == nil || ctx.Err() != nil || !bytes.Contains(output, []byte("--- FAIL: TestClampUpperBoundary (")) || bytes.Contains(output, []byte("panic:")) {
		return false
	}
	for _, line := range lines {
		if bytes.Contains(output, []byte(fmt.Sprintf("clamp_test.go:%d:", line))) {
			return true
		}
	}
	return false
}
