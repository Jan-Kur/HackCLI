package channel

import "strings"

func (a *app) findNextVisibleMessage(currentIndex int, goingDown bool) int {
	if goingDown {
		for i := currentIndex + 1; i < len(a.messages); i++ {
			if a.isVisible(a.messages[i]) {
				return i
			}
		}
	} else {
		for i := currentIndex - 1; i >= 0; i-- {
			if a.isVisible(a.messages[i]) {
				return i
			}
		}
	}
	return -1
}

func (a *app) isVisible(mes message) bool {
	if mes.threadId != "" && mes.ts != mes.threadId {
		for _, m := range a.messages {
			if m.ts == mes.threadId {
				if m.isCollapsed {
					return false
				}
			}
		}
	}
	return true
}

func (a *app) getMessageHeight(mes message) int {
	formattedMsg, _ := a.formatMessage(mes)

	return strings.Count(formattedMsg, "\n") + 1
}
