package tui

import (
	"strings"
	"unicode/utf8"
)

func (component *DetailStreamComponent) Append(value string) {
	line := Sanitize(value)
	if len(line) > maxVisibleStreamBytes {
		start := len(line) - maxVisibleStreamBytes
		for start < len(line) && !utf8.RuneStart(line[start]) {
			start++
		}
		line = line[start:]
	}
	component.streamLines = append(component.streamLines, line)
	component.streamBytes += len(line)
	for len(component.streamLines) > maxVisibleStreamEvents || component.streamBytes > maxVisibleStreamBytes {
		component.streamBytes -= len(component.streamLines[0])
		component.streamLines = component.streamLines[1:]
	}
	component.detail.SetContent(strings.Join(component.streamLines, "\n"))
	if component.autoscroll {
		component.detail.GotoBottom()
	}
}

func (component *DetailStreamComponent) Cancel() {
	if component.streamCancel != nil {
		component.streamCancel()
		component.streamCancel = nil
	}
	component.streamEvents = nil
	component.connected = false
}

func (component *DetailStreamComponent) StreamContent() string {
	connection := "disconnected"
	if component.connected {
		connection = "connected"
	}
	autoscroll := "off"
	if component.autoscroll {
		autoscroll = "on"
	}
	return "stream: " + connection + " · autoscroll: " + autoscroll + "\n" + component.detail.View()
}
