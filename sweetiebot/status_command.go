package sweetiebot

import (
	"strings"

	"github.com/bwmarrin/discordgo"
)

type SetStatusCommand struct {
}

func (c *SetStatusCommand) Name() string {
	return "SetStatus"
}
func (c *SetStatusCommand) Process(args []string, msg *discordgo.Message, info *GuildInfo) (string, bool) {
	if len(args) < 1 {
		sb.dg.UpdateStatus(0, "")
		return "```Removed status```", false
	}
	arg := strings.Join(args, " ")
	sb.dg.UpdateStatus(0, arg)
	return "```Set status to " + arg + "```", false
}
func (c *SetStatusCommand) Usage(info *GuildInfo) string {
	return info.FormatUsage(c, "[arbitrary string]", "Sets Sweetie Bot's status message to the given string, at least until she automatically changes it again.")
}
func (c *SetStatusCommand) UsageShort() string { return "Sets the status message." }
