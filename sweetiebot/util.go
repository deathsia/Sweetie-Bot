package sweetiebot

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func Pluralize(i int64, s string) string {
	if i == 1 {
		return strconv.FormatInt(i, 10) + s
	}
	return strconv.FormatInt(i, 10) + s + "s"
}

func TimeDiff(d time.Duration) string {
	seconds := int64(d.Seconds())
	if seconds <= 60 {
		return Pluralize(seconds, " second")
	}
	if seconds <= 60*60 {
		return Pluralize((seconds+1)/60, " minute")
	}
	days := (seconds + 100) / 86400
	hours := (seconds + 10 - (days * 86400)) / 3600
	minutes := (seconds - (days * 86400) - (hours * 3600)) / 60

	if days == 0 && minutes > 2 {
		return Pluralize(hours, " hour") + " and " + Pluralize(minutes, " minute")
	}
	if days == 0 {
		return Pluralize(hours, " hour")
	}
	if hours > 1 {
		return Pluralize(days, " day") + " and " + Pluralize(hours, " hour")
	}
	return Pluralize(days, " day")
}

func PingAtoi(s string) uint64 {
	if len(s) > 2 && (s[:2] == "<#" || s[:2] == "<@") {
		return SBatoi(s[2 : len(s)-1])
	}
	return SBatoi(s)
}
func StripPing(s string) string {
	if len(s) >= 2 && (s[:2] == "<#" || s[:2] == "<@") {
		if len(s) >= 3 && (s[2:3] == "!" || s[2:3] == "&") {
			return s[3 : len(s)-1]
		}
		return s[2 : len(s)-1]
	}
	return s
}
func SBatoi(s string) uint64 {
	if len(s) < 1 {
		return 0
	}
	if s[:1] == "!" || s[:1] == "&" {
		s = s[1:]
	}
	i, err := strconv.ParseUint(strings.Replace(s, "\u200B", "", -1), 10, 64)
	if err != nil {
		fmt.Println("Invalid number ", s, ":", err.Error())
		return 0
	}
	return i
}
func SBitoa(i uint64) string {
	return strconv.FormatUint(i, 10)
}
func IsSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r'
}

func IDsToUsernames(IDs []uint64, info *GuildInfo) []string {
	s := make([]string, 0, len(IDs))
	gid := SBatoi(info.Guild.ID)
	for _, v := range IDs {
		m, _ := sb.db.GetMember(v, gid)
		if m != nil {
			if len(m.Nick) > 0 {
				s = append(s, m.Nick)
			} else {
				s = append(s, m.User.Username)
			}
		} else {
			s = append(s, "<@"+SBitoa(v)+">")
		}
	}
	return s
}
func ParseArguments(s string) []string {
	r := []string{}
	l := len(s)
	for i := 0; i < l; i++ {
		c := s[i]
		if !IsSpace(c) {
			var start int

			if c == '"' {
				i++
				start = i
				for i < (l-1) && (s[i] != '"' || !IsSpace(s[i+1])) {
					i++
				}
			} else {
				start = i
				i++
				for i < l && !IsSpace(s[i]) {
					i++
				}
			}
			r = append(r, s[start:i])
		}
	}
	return r
}

// This constructs an XOR operator for booleans
func boolXOR(a bool, b bool) bool {
	return (a && !b) || (!a && b)
}

func (info *GuildInfo) UserHasRole(user string, role string) bool {
	m, err := sb.dg.GuildMember(info.Guild.ID, user)
	if err == nil {
		for _, v := range m.Roles {
			if v == role {
				return true
			}
		}
	}
	return false
}

func (info *GuildInfo) UserHasAnyRole(user string, roles map[string]bool) bool {
	if len(roles) == 0 {
		return true
	}
	m, err := sb.dg.GuildMember(info.Guild.ID, user)
	if err == nil {
		for _, v := range m.Roles {
			_, ok := roles[v]
			if ok {
				return true
			}
		}
	}
	return false
}

func ReadUserPingArg(args []string) (uint64, string) {
	if len(args) < 1 {
		return 0, "```You must provide a user to search for.```"
	}
	if len(args[0]) < 3 || args[0][0] != '<' || args[0][1] != '@' {
		return 0, "```The first argument must be an actual ping for the target user, not just their name typed out.```"
	}
	return SBatoi(args[0][2 : len(args[0])-1]), ""
}

func SinceUTC(t time.Time) time.Duration {
	return time.Now().UTC().Sub(t)
}

func getTimezone(info *GuildInfo, user *discordgo.User) *time.Location {
	if user != nil {
		loc := sb.db.GetTimeZone(SBatoi(user.ID))
		if loc != nil {
			return loc
		}
	}
	loc, err := time.LoadLocation(info.config.TimezoneLocation)
	if err == nil {
		return loc
	}
	return time.UTC
}
func ApplyTimezone(t time.Time, info *GuildInfo, user *discordgo.User) time.Time {
	return t.In(getTimezone(info, user))
}
func IngestEpisode(file string, season int, episode int) {
	f, err := ioutil.ReadFile(file)
	if err != nil {
		fmt.Println(err.Error())
	}
	s := strings.Split(strings.Replace(string(f), "\r", "", -1), "\n")

	songmode := false
	lastcharacter := ""
	adjust := 0
	for i := 0; i < len(s); i++ {
		if len(s[i]) > 0 {
			if s[i][0] == '[' {
				action := s[i][1 : len(s[i])-1]
				sb.db.AddTranscript(season, episode, i-adjust, "ACTION", action)
				if !songmode {
					lastcharacter = action
				}
			} else {
				split := strings.SplitN(s[i], ":", 2)
				songmode = (len(split) < 2)
				if songmode {
					prev := sb.db.GetTranscript(season, episode, i-1-adjust, i-1-adjust)
					if len(prev) != 1 {
						fmt.Println(season, " ", episode, " ", i-adjust)
						return
					}
					if prev[0].Speaker == "ACTION" && prev[0].Text == lastcharacter {
						adjust++
						sb.db.RemoveTranscript(season, episode, i-adjust)
					}
					sb.db.AddTranscript(season, episode, i-adjust, lastcharacter, strings.TrimSpace(split[0]))
				} else {
					lastcharacter = strings.TrimSpace(split[0])
					sb.db.AddTranscript(season, episode, i-adjust, lastcharacter, strings.TrimSpace(split[1]))
				}
			}
		} else {
			sb.db.AddTranscript(season, episode, i-adjust, "ACTION", "")
		}
	}
}

func SplitSpeaker(speaker string) []string {
	speakers := strings.Split(strings.Replace(speaker, ", and", " and", -1), " and ")
	speakers = append(strings.Split(speakers[0], ","), speakers[1:]...)
	for i, s := range speakers {
		speakers[i] = strings.Trim(strings.TrimSpace(strings.Replace(s, "Young", "", -1)), "\"")
	}
	return speakers
}

func BuildMarkov(season_start int, episode_start int) {
	regex := regexp.MustCompile("[^~!@#$%^&*()_+`=[\\];,./<>?\" \n\r\f\t\v]+[?!.]?")

	sb.db.sql_ResetMarkov.Exec()

	var cur uint64
	var prev uint64
	var prev2 uint64
	for season := season_start; season <= 5; season++ {
		for episode := episode_start; episode <= 26; episode++ {
			fmt.Println("Begin Episode", episode, "Season", season)
			prev = 0
			prev2 = 0
			lines := sb.db.GetTranscript(season, episode, 0, 999999)
			//lines := []Transcript{ {1, 1, 1, "Twilight", "Twilight went to the bakery to buy some cakes."}, {1, 1, 1, "Twilight", "Twilight went to the library to buy some books"} }
			fmt.Println("Got", len(lines), "lines")

			for i := 0; i < len(lines); i++ {
				if len(lines[i].Text) == 0 {
					if lines[i].Speaker != "ACTION" {
						fmt.Println("UNKNOWN SPEAKER: ", lines[i].Speaker)
					}
					cur = sb.db.AddMarkov(prev, prev2, lines[i].Speaker, "")
					prev2 = 0
					prev = cur // Cur will always be 0 here.
					continue
				}
				words := regex.FindAllString(lines[i].Text, -1)
				speakers := SplitSpeaker(lines[i].Speaker)
				for _, speaker := range speakers {
					if len(speaker) == 0 {
						fmt.Println("EMPTY SPEAKER GENERATED FROM \""+lines[i].Speaker+"\" ON LINE: ", lines[i].Text)
						fmt.Println(speakers)
					}
					for j, _ := range words {
						l := len(words[j])
						ch := words[j][l-1]
						switch ch {
						case '.', '!', '?':
							words[j] = words[j][:l-1]
						}
						if sb.db.GetMarkovWord(speaker, words[j]) != words[j] {
							words[j] = strings.ToLower(words[j])
						}
						//fmt.Println("AddMarkov: ", prev, prev2, speaker, words[j])
						cur = sb.db.AddMarkov(prev, prev2, speaker, words[j])
						prev2 = prev
						prev = cur

						switch ch {
						case '.', '!', '?':
							//fmt.Println("AddMarkov: ", prev, prev2, speaker, string(ch))
							cur = sb.db.AddMarkov(prev, prev2, speaker, string(ch))
							prev2 = 0
							prev = 0
							//prev = sb.db.AddMarkov(prev, "ACTION", "")
						}
					}
				}
			}
		}
	}
}

func FindUsername(user string, info *GuildInfo) []uint64 {
	if len(user) <= 0 {
		return []uint64{}
	}
	if userregex.MatchString(user) {
		return []uint64{SBatoi(user[2 : len(user)-1])}
	}
	if user[len(user)-1] == '@' {
		user = user[:len(user)-1]
	} else {
		user = "%" + user + "%"
	}
	r := sb.db.FindGuildUsers(user, 20, 0, SBatoi(info.Guild.ID))
	if len(r) != 0 {
		return r
	}
	return sb.db.FindUsers(user, 20, 0)
}

func GetCommandsInOrder(m map[string]Command) []string {
	s := make([]string, 0, len(m))
	for k, _ := range m {
		s = append(s, k)
	}
	sort.Strings(s)
	return s
}

func MapGetRandomItem(m map[string]bool) string {
	index := rand.Intn(len(m))
	for k, _ := range m {
		if index == 0 {
			return k
		}
		index--
	}

	return "SOMETHING IMPOSSIBLE HAPPENED IN UTIL.GO MapGetRandomItem()! Somebody drag Cloud Hop out of bed and tell him his bot is broken."
}

func MapToSlice(m map[string]bool) []string {
	s := make([]string, 0, len(m))
	for k, _ := range m {
		s = append(s, k)
	}
	return s
}

func MapIntToSlice(m map[int]string) []int {
	s := make([]int, 0, len(m))
	for k, _ := range m {
		s = append(s, k)
	}
	return s
}

func MapStringToSlice(m map[string]string) []string {
	s := make([]string, 0, len(m))
	for k, _ := range m {
		s = append(s, k)
	}
	return s
}

func RemoveSliceString(s *[]string, item string) bool {
	for i := 0; i < len(*s); i++ {
		if (*s)[i] == item {
			*s = append((*s)[:i], (*s)[i+1:]...)
			return true
		}
	}
	return false
}

func RemoveSliceInt(s *[]uint64, item uint64) bool {
	for i := 0; i < len(*s); i++ {
		if (*s)[i] == item {
			*s = append((*s)[:i], (*s)[i+1:]...)
			return true
		}
	}
	return false
}

func CheckMapNilBool(m *map[string]bool) {
	if len(*m) <= 0 {
		*m = make(map[string]bool)
	}
}

func CheckMapNilString(m *map[string]string) {
	if len(*m) <= 0 {
		*m = make(map[string]string)
	}
}

func FindIntSlice(item uint64, s []uint64) bool {
	for _, v := range s {
		if v == item {
			return true
		}
	}
	return false
}
func getUserName(user uint64, info *GuildInfo) string {
	m, _ := sb.db.GetMember(user, SBatoi(info.Guild.ID))
	if m == nil {
		return "<@" + SBitoa(user) + ">"
	}
	if len(m.Nick) > 0 {
		return m.Nick
	}
	return m.User.Username
}
func replacementionhelper(s string) string {
	u, _, _, _ := sb.db.GetUser(SBatoi(StripPing(s)))
	if u == nil {
		return s
	}
	return u.Username
}
func replacerolementionhelper(s string) string {
	return "<@\u200B&" + s[3:]
}

func ReplaceAllMentions(s string) string {
	s = userregex.ReplaceAllStringFunc(s, replacementionhelper)
	return roleregex.ReplaceAllStringFunc(s, replacerolementionhelper)
}
func RestrictCommand(v string, guild *GuildInfo) {
	_, ok := guild.config.Command_roles[v]
	if !ok && guild.config.AlertRole != 0 {
		guild.config.Command_roles[v] = make(map[string]bool)
		guild.config.Command_roles[v][SBitoa(guild.config.AlertRole)] = true
	}
}

// migrate settings from earlier config version
func MigrateSettings(guild *GuildInfo) {
	if guild.config.Version == 0 {
		newcommands := []string{"addevent", "addbirthday", "autosilence", "silence", "unsilence", "wipewelcome"}
		if len(guild.config.Command_roles) == 0 {
			guild.config.Command_roles = make(map[string]map[string]bool)
		}
		for _, v := range newcommands {
			RestrictCommand(v, guild)
		}
		guild.config.MaxImageSpam = 3
		guild.config.MaxAttachSpam = 1
		guild.config.MaxPingSpam = 24
		guild.config.MaxMessageSpam = make(map[int64]int)
		guild.config.MaxMessageSpam[1] = 4
		guild.config.MaxMessageSpam[9] = 10
		guild.config.MaxMessageSpam[12] = 15
	}

	if guild.config.Version == 1 {
		if len(guild.config.Aliases) == 0 {
			guild.config.Aliases = make(map[string]string)
		}
		guild.config.Aliases["cute"] = "pick cute"
		RestrictCommand("new", guild)
		RestrictCommand("addquote", guild)
		RestrictCommand("removequote", guild)
	}

	if guild.config.Version == 2 {
		RestrictCommand("removealias", guild)
	}

	if guild.config.Version == 3 {
		guild.config.BoredCommands = make(map[string]bool)
	}

	if guild.config.Version == 4 {
		RestrictCommand("delete", guild)
	}

	if guild.config.Version == 5 {
		guild.config.TimezoneLocation = "Etc/GMT"
		if guild.config.Timezone < 0 {
			guild.config.TimezoneLocation += "+"
		}
		guild.config.TimezoneLocation += strconv.Itoa(-guild.config.Timezone) // Etc has the sign reversed
	}

	if guild.config.Version == 6 {
		RestrictCommand("createpoll", guild)
		RestrictCommand("deletepoll", guild)
	}
	if guild.config.Version == 7 {
		RestrictCommand("addoption", guild)
	}

	if guild.config.Version != 8 {
		guild.config.Version = 8 // set version to most recent config version
		guild.SaveConfig()
	}
}

func parseCommonTime(s string, info *GuildInfo, user *discordgo.User) (time.Time, error) {
	t, err := time.ParseInLocation("_2 Jan 06 3:04pm -0700", s, locUTC)
	tz := getTimezone(info, user)
	if err != nil {
		t, err = time.ParseInLocation("_2 Jan 2006 3:04pm -0700", s, tz)
	}
	if err != nil {
		t, err = time.ParseInLocation("_2 Jan 2006 15:04 -0700", s, tz)
	}
	if err != nil {
		t, err = time.ParseInLocation("Jan _2 2006 3:04pm", s, tz)
	}
	if err != nil {
		t, err = time.ParseInLocation("Jan _2 2006 15:04", s, tz)
	}
	if err != nil {
		t, err = time.ParseInLocation("Jan _2 2006", s, tz)
	}
	if err != nil {
		t, err = time.ParseInLocation("Jan _2 3:04pm", s, tz)
		if err == nil {
			t = t.AddDate(ApplyTimezone(time.Now().UTC(), info, user).Year(), 0, 0)
		}
	}
	if err != nil {
		t, err = time.ParseInLocation("Jan _2 15:04", s, tz)
		if err == nil {
			t = t.AddDate(ApplyTimezone(time.Now().UTC(), info, user).Year(), 0, 0)
		}
	}
	if err != nil {
		t, err = time.ParseInLocation("Jan _2", s, tz)
		if err == nil {
			t = t.AddDate(ApplyTimezone(time.Now().UTC(), info, user).Year(), 0, 0)
		}
	}
	if err != nil {
		t, err = time.ParseInLocation("_2 Jan 06 3:04pm", s, tz)
	}
	if err != nil {
		t, err = time.ParseInLocation("_2 Jan 06", s, tz)
	}
	return t, err
}

func getAllPerms(info *GuildInfo, user string) (int64, error) {
	for _, v := range info.Guild.Members {
		if v.User.ID == user {
			var perms int64 = 0
			for _, r := range v.Roles {
				for _, x := range info.Guild.Roles {
					if x.ID == r {
						perms |= int64(x.Permissions)
					}
				}
			}
			return perms, nil
		}
	}
	return 0, errors.New("Cannot find member!")
}

func findServers(name string, guilds []uint64) []*GuildInfo {
	name = strings.ToLower(name)
	info := make([]*GuildInfo, 0, len(guilds))
	for _, g := range guilds {
		guild, ok := sb.guilds[g]
		if ok {
			n := strings.ToLower(guild.Guild.Name)
			if len(n) > 0 {
				if n == name { // if these are an EXACT match, throw away the other results and just return this
					return []*GuildInfo{guild}
				}
				if strings.Contains(n, name) {
					info = append(info, guild)
				}
			} else {
				info = append(info, guild)
			}
		}
	}
	return info
}

func getDefaultServer(user uint64) *GuildInfo {
	_, _, _, server := sb.db.GetUser(user)
	if server == nil {
		return nil
	}
	info, ok := sb.guilds[*server]
	if !ok {
		return nil
	}
	return info
}
