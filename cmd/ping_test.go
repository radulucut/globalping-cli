package cmd

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/jsdelivr/globalping-cli/globalping"
	"github.com/jsdelivr/globalping-cli/mocks"
	"github.com/jsdelivr/globalping-cli/view"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func Test_Execute_Ping_Default(t *testing.T) {
	t.Cleanup(sessionCleanup)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedOpts := getMeasurementCreate()
	expectedResponse := getMeasurementCreateResponse(measurementID1)

	gbMock := mocks.NewMockClient(ctrl)
	gbMock.EXPECT().CreateMeasurement(expectedOpts).Times(1).Return(expectedResponse, false, nil)

	viewerMock := mocks.NewMockViewer(ctrl)
	viewerMock.EXPECT().Output(measurementID1, expectedOpts).Times(1).Return(nil)

	ctx = &view.Context{
		MaxHistory: 1,
	}
	r, w, err := os.Pipe()
	assert.NoError(t, err)
	defer r.Close()
	defer w.Close()

	printer := view.NewPrinter(w)
	root := NewRoot(w, w, printer, ctx, viewerMock, gbMock, &cobra.Command{})
	os.Args = []string{"globalping", "ping", "jsdelivr.com"}
	err = root.Cmd.ExecuteContext(context.TODO())
	assert.NoError(t, err)
	w.Close()

	output, err := io.ReadAll(r)
	assert.NoError(t, err)
	assert.Equal(t, "", string(output))

	expectedCtx := getExpectedViewContext()
	assert.Equal(t, expectedCtx, ctx)

	b, err := os.ReadFile(getMeasurementsPath())
	assert.NoError(t, err)
	expectedHistory := []byte(measurementID1 + "\n")
	assert.Equal(t, expectedHistory, b)
}

func Test_Execute_Ping_Infinite(t *testing.T) {
	t.Cleanup(sessionCleanup)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedOpts1 := getMeasurementCreate()
	expectedOpts1.Options.Packets = 16
	expectedOpts2 := getMeasurementCreate()
	expectedOpts2.Options.Packets = 16
	expectedOpts2.Locations[0].Magic = measurementID1
	expectedOpts3 := getMeasurementCreate()
	expectedOpts3.Options.Packets = 16
	expectedOpts3.Locations[0].Magic = measurementID2

	expectedResponse1 := getMeasurementCreateResponse(measurementID1)
	expectedResponse2 := getMeasurementCreateResponse(measurementID2)
	expectedResponse3 := getMeasurementCreateResponse(measurementID3)

	gbMock := mocks.NewMockClient(ctrl)
	call1 := gbMock.EXPECT().CreateMeasurement(expectedOpts1).Return(expectedResponse1, false, nil)
	call2 := gbMock.EXPECT().CreateMeasurement(expectedOpts2).Return(expectedResponse2, false, nil).After(call1)
	gbMock.EXPECT().CreateMeasurement(expectedOpts3).Return(expectedResponse3, false, nil).After(call2)

	viewerMock := mocks.NewMockViewer(ctrl)
	outputCall1 := viewerMock.EXPECT().OutputInfinite(measurementID1).DoAndReturn(func(id string) error {
		time.Sleep(5 * time.Millisecond)
		return nil
	})
	outputCall2 := viewerMock.EXPECT().OutputInfinite(measurementID2).DoAndReturn(func(id string) error {
		time.Sleep(5 * time.Millisecond)
		return nil
	}).After(outputCall1)
	viewerMock.EXPECT().OutputInfinite(measurementID3).DoAndReturn(func(id string) error {
		time.Sleep(5 * time.Millisecond)
		return nil
	}).After(outputCall2)

	viewerMock.EXPECT().OutputSummary().Times(1)

	ctx = &view.Context{}
	r, w, err := os.Pipe()
	assert.NoError(t, err)
	defer r.Close()
	defer w.Close()

	printer := view.NewPrinter(w)
	root := NewRoot(w, w, printer, ctx, viewerMock, gbMock, &cobra.Command{})
	os.Args = []string{"globalping", "ping", "jsdelivr.com", "--infinite"}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT)
	go func() {
		time.Sleep(14 * time.Millisecond)
		p, _ := os.FindProcess(os.Getpid())
		p.Signal(os.Interrupt)
	}()
	err = root.Cmd.ExecuteContext(context.TODO())
	<-sig

	assert.NoError(t, err)
	w.Close()

	output, err := io.ReadAll(r)
	assert.NoError(t, err)
	assert.Equal(t, "", string(output))

	expectedCtx := &view.Context{
		Cmd:       "ping",
		Target:    "jsdelivr.com",
		Limit:     1,
		CI:        true,
		Infinite:  true,
		CallCount: 3,
		From:      measurementID2,
		Packets:   16,
	}
	assert.Equal(t, expectedCtx, ctx)

	b, err := os.ReadFile(getMeasurementsPath())
	assert.NoError(t, err)
	expectedHistory := []byte(measurementID1 + "\n")
	assert.Equal(t, expectedHistory, b)
}

func getExpectedViewContext() *view.Context {
	return &view.Context{
		Cmd:        "ping",
		Target:     "jsdelivr.com",
		Limit:      1,
		CI:         true,
		CallCount:  1,
		From:       "world",
		MaxHistory: 1,
	}
}

func getMeasurementCreate() *globalping.MeasurementCreate {
	return &globalping.MeasurementCreate{
		Type:    "ping",
		Target:  "jsdelivr.com",
		Limit:   1,
		Options: &globalping.MeasurementOptions{},
		Locations: []globalping.Locations{
			{Magic: "world"},
		},
	}
}

func getMeasurementCreateResponse(id string) *globalping.MeasurementCreateResponse {
	return &globalping.MeasurementCreateResponse{
		ID:          id,
		ProbesCount: 1,
	}
}
