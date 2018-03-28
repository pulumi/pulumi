// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	// "math/rand"
	"os"
	// "strconv"
	"time"
	//	"time"
	// "github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/term"
	// "github.com/pulumi/pulumi/pkg/backend"
	// "github.com/pulumi/pulumi/pkg/engine"
	// "github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

// copied from: https://github.com/docker/cli/blob/master/cli/command/out.go
// replace with usage of that library when we can figure out hte right version story

type commonStream struct {
	fd         uintptr
	isTerminal bool
	state      *term.State
}

// FD returns the file descriptor number for this stream
func (s *commonStream) FD() uintptr {
	return s.fd
}

// IsTerminal returns true if this stream is connected to a terminal
func (s *commonStream) IsTerminal() bool {
	return s.isTerminal
}

// RestoreTerminal restores normal mode to the terminal
func (s *commonStream) RestoreTerminal() {
	if s.state != nil {
		term.RestoreTerminal(s.fd, s.state)
	}
}

// SetIsTerminal sets the boolean used for isTerminal
func (s *commonStream) SetIsTerminal(isTerminal bool) {
	s.isTerminal = isTerminal
}

type outStream struct {
	commonStream
	out io.Writer
}

func (o *outStream) Write(p []byte) (int, error) {
	return o.out.Write(p)
}

// SetRawTerminal sets raw mode on the input terminal
func (o *outStream) SetRawTerminal() (err error) {
	if os.Getenv("NORAW") != "" || !o.commonStream.isTerminal {
		return nil
	}
	o.commonStream.state, err = term.SetRawTerminalOutput(o.commonStream.fd)
	return err
}

// GetTtySize returns the height and width in characters of the tty
func (o *outStream) GetTtySize() (uint, uint) {
	if !o.isTerminal {
		return 0, 0
	}
	ws, err := term.GetWinsize(o.fd)
	if err != nil {
		if ws == nil {
			return 0, 0
		}
	}
	return uint(ws.Height), uint(ws.Width)
}

// NewOutStream returns a new OutStream object from a Writer
func newOutStream(out io.Writer) *outStream {
	fd, isTerminal := term.GetFdInfo(out)
	return &outStream{commonStream: commonStream{fd: fd, isTerminal: isTerminal}, out: out}
}

func writeDistributionProgress(outStream io.Writer, progressChan <-chan progress.Progress) {
	progressOutput := streamformatter.NewJSONStreamFormatter().NewProgressOutput(outStream, false)

	for prog := range progressChan {
		// fmt.Printf("Received progress")
		progressOutput.WriteProgress(prog)
	}
}

// end of copy

// func newOutStream(out io.Writer) *OutStream {
// 	fd, isTerminal := term.GetFdInfo(out)
// 	return &OutStream{CommonStream: CommonStream{fd: fd, isTerminal: isTerminal}, out: out}
// }

func newUpdate2Cmd() *cobra.Command {
	var debug bool
	var stack string

	var message string

	// Flags for engine.UpdateOptions.
	var analyzers []string
	var color colorFlag
	var parallel int
	var preview bool
	var showConfig bool
	var showReplacementSteps bool
	var showSames bool
	var summary bool

	var cmd = &cobra.Command{
		Use:     "update2",
		Aliases: []string{"up2"},
		Short:   "New update",
		Long:    "New update",
		Args:    cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			_, stdout, _ := term.StdStreams()
			_, isTerminal := term.GetFdInfo(stdout)

			pipeReader, pipeWriter := io.Pipe()
			progressChan := make(chan progress.Progress, 100)

			chanOutput := progress.ChanOutput(progressChan)

			go func() {
				writeDistributionProgress(pipeWriter, progressChan)
				pipeWriter.Close()
			}()

			// progressBuckets := make(map[int64]int64)

			// go func() {
			// 	for {
			// 		bucket := rand.Int63() % 5
			// 		val, has := progressBuckets[bucket]

			// 		if has {
			// 			val++
			// 		} else {
			// 			val = 0
			// 		}

			// 		progressBuckets[bucket] = val
			// 		id := fmt.Sprintf("%v", bucket)
			// 		if val >= 100 {
			// 			chanOutput.WriteProgress(progress.Progress{
			// 				ID:     id,
			// 				Action: "Done!",
			// 			})
			// 		} else {
			// 			chanOutput.WriteProgress(progress.Progress{
			// 				ID:      id,
			// 				Action:  "updating...",
			// 				Current: val,
			// 				Total:   100,
			// 			})
			// 		}

			// 		time.Sleep(time.Millisecond * 100)
			// 	}
			// }()

			go func() {
				file, _ := ioutil.ReadFile("/home/cyrusn/Downloads/video-thumbnailer.checkpoint.initial.json")
				var topLevelObj map[string]interface{}
				json.Unmarshal(file, &topLevelObj)
				checkpointObj := topLevelObj["checkpoint"].(map[string]interface{})
				latestObj := checkpointObj["latest"].(map[string]interface{})
				resourcesArray := latestObj["resources"].([]interface{})

				var stackUrn string
				var endTime = time.Unix(1<<63-62135596801, 999999999)
				var nextTime = endTime

				var topLevelResourceToChildrenWithTimes = make(map[string][]string)

				var getTopLevelResource func(r map[string]interface{}) map[string]interface{}
				getTopLevelResource = func(r map[string]interface{}) map[string]interface{} {
					parent := r["parent"].(string)
					if parent == stackUrn {
						return r
					}

					for _, resourceObjAny := range resourcesArray {
						resourceObj := resourceObjAny.(map[string]interface{})
						resourceUrn := resourceObj["urn"].(string)
						if resourceUrn == parent {
							return getTopLevelResource(resourceObj)
						}
					}

					panic(fmt.Sprintf("Could not find parent: %s", parent))
				}

				for _, resourceObjAny := range resourcesArray {
					resourceObj := resourceObjAny.(map[string]interface{})
					resourceType := resourceObj["type"].(string)
					urn := resourceObj["urn"].(string)
					if resourceType == "pulumi:pulumi:Stack" {
						stackUrn = urn
						continue
					}

					lastUpdateStartTime := resourceObj["lastUpdateStartTime"].(string)

					if lastUpdateStartTime != "0001-01-01T00:00:00Z" {
						lastUpdate, e := time.Parse(time.RFC3339Nano, lastUpdateStartTime)
						if e != nil {
							panic(e)
						}

						if lastUpdate.Before(nextTime) {
							nextTime = lastUpdate
						}

						topLevelResource := getTopLevelResource(resourceObj)
						topLevelResourceUrn := topLevelResource["urn"].(string)

						children := topLevelResourceToChildrenWithTimes[topLevelResourceUrn]
						children = append(children, urn)
						topLevelResourceToChildrenWithTimes[topLevelResourceUrn] = children
					}
				}

				// for k, v := range topLevelResourceToChildrenWithTimes {
				// 	fmt.Printf("%s\n", k)
				// 	for _, c := range v {
				// 		fmt.Printf("  %s\n", c)
				// 	}
				// }

				for nextTime != endTime {
					nextNextTime := endTime

					var toStart []map[string]interface{}
					var toEnd []map[string]interface{}

					for _, resourceObjAny := range resourcesArray {
						resourceObj := resourceObjAny.(map[string]interface{})
						lastUpdateStartTime := resourceObj["lastUpdateStartTime"].(string)
						if lastUpdateStartTime != "0001-01-01T00:00:00Z" {
							startTime, _ := time.Parse(time.RFC3339Nano, lastUpdateStartTime)

							lastUpdateEndTime := resourceObj["lastUpdateEndTime"].(string)
							endTime, _ := time.Parse(time.RFC3339Nano, lastUpdateEndTime)

							if startTime == nextTime {
								toStart = append(toStart, resourceObj)
							} else if startTime.After(nextTime) && startTime.Before(nextNextTime) {
								nextNextTime = startTime
							}

							if endTime == nextTime {
								toEnd = append(toEnd, resourceObj)
							} else if endTime.After(nextTime) && endTime.Before(nextNextTime) {
								nextNextTime = endTime
							}
						}
					}

					getResourceName := func(r map[string]interface{}) string {
						resourceType := r["type"].(string)
						resourceInputs, e := r["inputs"].(map[string]interface{})
						var name = ""
						if e {
							name, e = resourceInputs["name"].(string)
						}

						if name == "" {
							name, e = r["id"].(string)
						}

						if name != "" {
							return fmt.Sprintf("%s(\"%s\")", resourceType, name)
						}

						return resourceType
					}

					for _, start := range toStart {
						startUrn := start["urn"].(string)
						if isTerminal {
							topLevelResource := getTopLevelResource(start)
							topLevelUrn := topLevelResource["urn"].(string)
							var action string
							if startUrn == topLevelUrn {
								action = "\033[33mStarting...\033[0m"
							} else {
								action = fmt.Sprintf("\033[32mCreating %s\033[0m", getResourceName(start))
							}

							chanOutput.WriteProgress(progress.Progress{
								ID:     getResourceName(topLevelResource),
								Action: action,
							})
						} else {
							chanOutput.WriteProgress(progress.Progress{
								ID:      getResourceName(start),
								Message: "Creating...",
							})
						}
					}

					remove := func(a []string, val string) []string {
						for i := range a {
							if a[i] == val {
								a[i] = a[len(a)-1] // Replace it with the last one.
								a = a[:len(a)-1]   // Chop off the last one.
								return a
							}
						}

						panic(fmt.Sprintf("Could not file %s", val))
					}

					for _, end := range toEnd {
						if isTerminal {
							topLevelResource := getTopLevelResource(end)
							topLevelUrn := topLevelResource["urn"].(string)
							endUrn := end["urn"].(string)

							children := topLevelResourceToChildrenWithTimes[topLevelUrn]
							children = remove(children, endUrn)
							topLevelResourceToChildrenWithTimes[topLevelUrn] = children

							var action string
							if len(children) > 0 {
								action = fmt.Sprintf("\033[32mFinished %s. \033[33mWaiting...\033[0m", getResourceName(end))
							} else {
								action = "\033[34mDone!\033[0m"
							}

							chanOutput.WriteProgress(progress.Progress{
								ID:     getResourceName(topLevelResource),
								Action: action,
							})
						} else {
							chanOutput.WriteProgress(progress.Progress{
								ID:      getResourceName(end),
								Message: "\033[34mDone!\033[0m",
							})
						}
					}

					if nextNextTime != endTime {
						time.Sleep(nextNextTime.Sub(nextTime))
					}

					nextTime = nextNextTime
				}

				close(progressChan)
			}()

			e := jsonmessage.DisplayJSONMessagesToStream(pipeReader, newOutStream(stdout), nil)
			fmt.Printf("\033[34mAll Done!\033[0m\n")

			return e

			// s, err := requireStack(tokens.QName(stack), true)
			// if err != nil {
			// 	return err
			// }
			// proj, root, err := readProject()
			// if err != nil {
			// 	return err
			// }

			// m, err := getUpdateMetadata(message, root)
			// if err != nil {
			// 	return errors.Wrap(err, "gathering environment metadata")
			// }

			// return s.Update(proj, root, debug, m, engine.UpdateOptions{
			// 	Analyzers: analyzers,
			// 	DryRun:    preview,
			// 	Parallel:  parallel,
			// 	Debug:     debug,
			// }, backend.DisplayOptions{
			// 	Color:                color.Colorization(),
			// 	ShowConfig:           showConfig,
			// 	ShowReplacementSteps: showReplacementSteps,
			// 	ShowSames:            showSames,
			// 	Summary:              summary,
			// })
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&debug, "debug", "d", false,
		"Print detailed debugging output during resource operations")
	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"Choose a stack other than the currently selected one")

	cmd.PersistentFlags().StringVarP(
		&message, "message", "m", "",
		"Optional message to associate with the update operation")

	// Flags for engine.UpdateOptions.
	cmd.PersistentFlags().VarP(
		&color, "color", "c", "Colorize output. Choices are: always, never, raw, auto")
	cmd.PersistentFlags().StringSliceVar(
		&analyzers, "analyzer", []string{},
		"Run one or more analyzers as part of this update")
	cmd.PersistentFlags().IntVarP(
		&parallel, "parallel", "p", 0,
		"Allow P resource operations to run in parallel at once (<=1 for no parallelism)")
	cmd.PersistentFlags().BoolVarP(
		&preview, "preview", "n", false,
		"Don't create/delete resources; just preview the planned operations")
	cmd.PersistentFlags().BoolVar(
		&showConfig, "show-config", false,
		"Show configuration keys and variables")
	cmd.PersistentFlags().BoolVar(
		&showReplacementSteps, "show-replacement-steps", false,
		"Show detailed resource replacement creates and deletes instead of a single step")
	cmd.PersistentFlags().BoolVar(
		&showSames, "show-sames", false,
		"Show resources that needn't be updated because they haven't changed, alongside those that do")
	cmd.PersistentFlags().BoolVar(
		&summary, "summary", false,
		"Only display summarization of resources and operations")

	return cmd
}
