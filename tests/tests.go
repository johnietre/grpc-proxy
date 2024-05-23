package tests

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"
)

func GetBinPath(release bool) string {
  dir := "debug"
  if release {
    dir = "release"
  }
  _, thisFile, _, _ := runtime.Caller(0)
  return filepath.Join(
    filepath.Dir(filepath.Dir(thisFile)),
    "target", dir, "grpc-proxy",
  )
}

func GetGoPath() string {
  _, thisFile, _, _ := runtime.Caller(0)
  return filepath.Join(
    filepath.Dir(filepath.Dir(thisFile)),
    "bin", "grpc-proxy",
  )
}

func GetConfigPath(name string) string {
  _, thisFile, _, _ := runtime.Caller(0)
  return filepath.Join(filepath.Dir(thisFile), "confs", name)
}

// GetPath gets a path relative to the "tests" directory.
func GetPath(parts ...string) string {
  _, thisFile, _, _ := runtime.Caller(0)
  parts = append([]string{filepath.Dir(thisFile)}, parts...)
  return filepath.Join(parts...)
}

func Timeout(dur time.Duration, f func()) bool {
  c := make(chan struct{})
  go func() {
    f()
    close(c)
  }()
  timer := time.NewTimer(dur)
  select {
  case <-c:
  case <-timer.C:
    return false
  }
  if !timer.Stop() {
    <-timer.C
  }
  return true
}

func TimeoutContext(dur time.Duration) (context.Context, func()) {
  return context.WithTimeout(context.Background(), dur)
}

type CmdName struct {
  Name string
  Cmd *exec.Cmd
}

func Wait(cmdNames ...CmdName) *sync.WaitGroup {
  wg := &sync.WaitGroup{}
  for _, cmdName := range cmdNames {
    wg.Add(1)
    go func(cmdName CmdName) {
      defer wg.Done()
      err := cmdName.Cmd.Wait()
      log.Println("FINISHED:", cmdName.Name)
      if err != nil {
        Fatal(err)
      }
    }(cmdName)
  }
  return wg
}

func NewCmd(ctx context.Context, name string, arg ...string) *exec.Cmd {
  cmd := exec.CommandContext(ctx, name, arg...)
  if cmd.SysProcAttr == nil {
    cmd.SysProcAttr = &syscall.SysProcAttr{}
  }
  cmd.SysProcAttr.Setpgid = true
  cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
  return cmd
}

func NewCommand(
  ctx context.Context,
  name string, arg ...string,
) (*exec.Cmd, OutputCapturer) {
  cmd, oc := NewCmd(ctx, name, arg...), NewOutputCapturer()
  cmd.Stdout, cmd.Stderr = oc.Stdout, oc.Stderr
  return cmd, oc
}

func NewCommandWithFiles(
  ctx context.Context,
  testName, progName string,
  name string, arg ...string,
) *exec.Cmd {
  cmd := NewCmd(ctx, name, arg...)
  cmd.Stdout, cmd.Stderr = NewOutErrFiles(testName, progName)
  return cmd
}

func NewOutErrFiles(testName, progName string) (*os.File, *os.File) {
  outF := Must(os.Create(GetPath(testName, progName+".stdout")))
  errF := Must(os.Create(GetPath(testName, progName+".stderr")))
  return outF, errF
}

type OutputCapturer struct {
  Stdout *bytes.Buffer
  Stderr *bytes.Buffer
}

func NewOutputCapturer() OutputCapturer {
  return OutputCapturer{
    Stdout: bytes.NewBuffer(nil),
    Stderr: bytes.NewBuffer(nil),
  }
}

func (oc *OutputCapturer) PrintStdout() {
  fmt.Print("=====STDOUT=====\n\n")
  os.Stdout.Write(oc.Stdout.Bytes())
  fmt.Print("\n===============\n")
}

func (oc *OutputCapturer) PrintStdoutIf() {
  if oc.Stdout.Len() == 0 {
    return
  }
  oc.PrintStdout()
}

func (oc *OutputCapturer) PrintStderr() {
  fmt.Print("=====STDERR=====\n\n")
  os.Stderr.Write(oc.Stderr.Bytes())
  fmt.Print("\n===============\n")
}

func (oc *OutputCapturer) PrintStderrIf() {
  if oc.Stderr.Len() == 0 {
    return
  }
  oc.PrintStderr()
}

func Fatal(args ...any) {
  args = append([]any{"FAILED\n"}, args...)
  log.Fatal(args...)
}

func Fatalf(format string, args ...any) {
  log.Fatalf("FAILED\n"+format, args...)
}

func Must[T any](t T, err error) T {
  if err != nil {
    Fatal(err)
  }
  return t
}

func TimeTest(name string, test func()) {
  start := time.Now()
  test()
  fmt.Printf("PASSED: %s (%f seconds)\n", name, time.Since(start).Seconds())
}

func TimedTest(dur time.Duration, name string, test func()) {
  start := time.Now()
  if !Timeout(dur, test) {
    Fatalf("%s: timed out (%f seconds)", name, dur.Seconds())
  }
  fmt.Printf("PASSED: %s (%f seconds)\n", name, time.Since(start).Seconds())
}
