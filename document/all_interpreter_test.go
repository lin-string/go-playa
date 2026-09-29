package document

import (
	"errors"
	"testing"
	"time"
)

func TestAsyncAllInputOwnsBorrowedOperands(t *testing.T) {
	scalarOperands := []Object{Number(1), Name("State")}
	scalar := newAllInput(newContentOpBorrowed("cm", scalarOperands, 0), false)
	scalarOperands[0] = Number(9)
	var scratch [6]Object
	gotScalar := scalar.operation(&scratch).operandsValue()
	if gotScalar[0] != Number(1) || gotScalar[1] != Name("State") {
		t.Fatalf("inline operands = %#v, want owned scalar values", gotScalar)
	}

	properties := Dict{Name("MCID"): Number(3)}
	compositeOperands := []Object{Name("Span"), properties}
	composite := newAllInput(newContentOpBorrowed("BDC", compositeOperands, 0), false)
	properties[Name("MCID")] = Number(8)
	gotComposite := composite.operation(&scratch).operandsValue()
	gotProperties, ok := gotComposite[1].(Dict)
	if !ok || gotProperties[Name("MCID")] != Number(3) {
		t.Fatalf("composite operands = %#v, want owned dictionary", gotComposite)
	}
}

func TestRunAllInterpreterWorkerReportsPanicAndCompletion(t *testing.T) {
	input := make(chan allInput)
	messages := make(chan allMessage, 2)
	state := allWorkerState{done: make(chan struct{})}
	go runAllInterpreterWorker(input, messages, &state, func(func() (ContentOp, bool), func(ContentObject)) error {
		panic("boom")
	})

	select {
	case <-state.done:
	case <-time.After(time.Second):
		t.Fatal("panicking worker did not publish completion")
	}
	if state.err == nil {
		t.Fatal("panicking worker did not retain its error")
	}
	if message := <-messages; message.kind != allError || message.err == nil {
		t.Fatalf("first worker message = %#v, want error", message)
	}
	if message := <-messages; message.kind != allDone {
		t.Fatalf("second worker message kind = %v, want done", message.kind)
	}
}

func TestSendAllInterpreterInputDoesNotBlockAfterWorkerExit(t *testing.T) {
	input := make(chan allInput, 1)
	input <- allInput{}
	done := make(chan struct{})
	close(done)
	if sendAllInterpreterInput(input, done, allInput{}) {
		t.Fatal("send succeeded after worker exit")
	}
}

func TestRunAllTextInterpreterRecoversInternalPanic(t *testing.T) {
	consumerYielding := false
	err := runAllTextInterpreter(&consumerYielding, func() {
		panic("boom")
	})
	if err == nil {
		t.Fatal("text interpreter panic was not converted to an error")
	}
}

func TestRunAllTextInterpreterPreservesConsumerPanic(t *testing.T) {
	want := errors.New("consumer panic")
	defer func() {
		if recovered := recover(); recovered != want {
			t.Fatalf("recovered panic = %v, want original consumer panic", recovered)
		}
	}()
	consumerYielding := false
	_ = runAllTextInterpreter(&consumerYielding, func() {
		yieldAllContentObject(func(ContentObject, error) bool {
			panic(want)
		}, &consumerYielding, ContentObject{}, nil)
	})
}

func TestWaitForAllInterpreterReadyPropagatesWorkerError(t *testing.T) {
	want := errors.New("worker failed")
	messages := []chan allMessage{
		make(chan allMessage, 1),
		make(chan allMessage, 1),
	}
	messages[0] <- allMessage{kind: allReady}
	messages[1] <- allMessage{kind: allError, err: want}

	got, done := waitForAllInterpreterReady(messages, []bool{true, true})
	if !errors.Is(got, want) || done {
		t.Fatalf("worker readiness result = err:%v done:%v, want err:%v done:false", got, done, want)
	}
}
