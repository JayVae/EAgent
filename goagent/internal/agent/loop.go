package agent

import (
	"context"
	"fmt"
)

// RunTask 发送消息并自动驱动工具循环：回复中包含工具调用就执行，
// 结果回传后再等下一轮，直到不再调用工具或达到轮数上限。
// 返回最终一条不含工具调用的回复。
func (a *Agent) RunTask(ctx context.Context, eng *Engine, msg string, files []string) (string, error) {
	current, currentFiles := msg, files
	for round := 0; ; round++ {
		if round >= a.cfg.MaxToolRounds {
			return "", fmt.Errorf("已连续执行 %d 轮工具，为防失控中止。可调大 max_tool_rounds", a.cfg.MaxToolRounds)
		}
		reply, err := a.Round(ctx, current, currentFiles)
		if reply != "" && err != nil {
			a.logf("警告: %v", err) // 部分回复也继续处理
		} else if err != nil {
			return "", err
		}
		feedback, attach, ran, perr := eng.HandleReply(ctx, reply)
		if perr != nil {
			feedback, ran = "!![工具][引擎] 解析/执行出错!!\n"+perr.Error(), true
		}
		if !ran {
			return reply, nil
		}
		current = "//[工具][取回结果]//\n" + feedback + "\n//[END]//"
		currentFiles = nil
		if attach != "" {
			currentFiles = []string{attach}
			current = "//[工具][取回结果]//\n" + feedback + "\n结果见附件。\n//[END]//"
		}
	}
}
