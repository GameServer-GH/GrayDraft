package provider

import "testing"

func TestStreamFinishedEmptyStringContinues(t *testing.T) {
	empty := ""
	if streamFinished(&empty) {
		t.Fatal("empty finish_reason must not end the stream")
	}
	if streamFinished(nil) {
		t.Fatal("nil finish_reason must not end the stream")
	}
	stop := "stop"
	if !streamFinished(&stop) {
		t.Fatal("non-empty finish_reason must end the stream")
	}
}
