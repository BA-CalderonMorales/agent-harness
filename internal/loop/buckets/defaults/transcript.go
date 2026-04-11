package defaults

// Transcript bucket defaults
const (
	// History limits
	TranscriptMaxHistoryDefault = 1000
	TranscriptMaxHistoryFast    = 100
	TranscriptMaxHistoryFull    = 10000

	// Search settings
	TranscriptMaxMatches    = 50
	TranscriptContextChars  = 200

	// Summary settings
	TranscriptSummaryMaxLength    = 2000
	TranscriptSummaryBulletPoints = 10

	// Topic extraction
	TranscriptMaxTopics     = 10
	TranscriptMinTopicFreq  = 2
)

// TranscriptStopWords contains words to ignore in topic extraction
var TranscriptStopWords = map[string]bool{
	// Articles
	"the": true, "a": true, "an": true,
	// Prepositions
	"in": true, "on": true, "at": true, "to": true, "for": true,
	"with": true, "about": true, "as": true, "into": true, "through": true,
	// Conjunctions
	"and": true, "or": true, "but": true, "so": true, "yet": true,
	// Pronouns
	"i": true, "you": true, "he": true, "she": true, "it": true,
	"we": true, "they": true, "me": true, "him": true, "her": true,
	"us": true, "them": true, "my": true, "your": true, "his": true,
	// Common verbs
	"is": true, "are": true, "was": true, "were": true, "be": true,
	"been": true, "being": true, "have": true, "has": true, "had": true,
	"do": true, "does": true, "did": true, "will": true, "would": true,
	"could": true, "should": true, "may": true, "might": true, "must": true,
	// Common words
	"can": true, "this": true, "that": true, "these": true, "those": true,
	"here": true, "there": true, "now": true, "then": true, "when": true,
	"where": true, "why": true, "how": true, "what": true, "which": true,
	"who": true, "whom": true, "whose": true, "all": true, "each": true,
	"every": true, "both": true, "few": true, "more": true, "most": true,
	"other": true, "some": true, "such": true, "no": true, "nor": true,
	"not": true, "only": true, "own": true, "same": true, "than": true,
	"too": true, "very": true, "just": true, "also": true, "get": true,
	"got": true, "go": true, "going": true, "went": true, "gone": true,
	"make": true, "made": true, "take": true, "took": true, "taken": true,
	"come": true, "came": true, "coming": true, "see": true, "saw": true,
	"seen": true, "know": true, "knew": true, "known": true, "think": true,
	"thought": true, "say": true, "said": true, "saying": true, "want": true,
	"wanted": true, "use": true, "used": true, "using": true, "find": true,
	"found": true, "finding": true, "give": true, "gave": true, "given": true,
	"giving": true, "tell": true, "told": true, "telling": true, "become": true,
	"became": true, "becoming": true, "leave": true, "left": true, "leaving": true,
	"feel": true, "felt": true, "feeling": true, "put": true, "putting": true,
	"mean": true, "meant": true, "meaning": true, "keep": true, "kept": true,
	"keeping": true, "let": true, "letting": true, "begin": true, "began": true,
	"begun": true, "beginning": true, "seem": true, "seemed": true, "seeming": true,
	"help": true, "helped": true, "helping": true, "show": true, "showed": true,
	"shown": true, "showing": true, "hear": true, "heard": true, "hearing": true,
	"play": true, "played": true, "playing": true, "run": true, "ran": true,
	"running": true, "move": true, "moved": true, "moving": true, "live": true,
	"lived": true, "living": true, "believe": true, "believed": true, "believing": true,
	"bring": true, "brought": true, "bringing": true, "happen": true, "happened": true,
	"happening": true, "write": true, "wrote": true, "written": true, "writing": true,
	"provide": true, "provided": true, "providing": true, "sit": true, "sat": true,
	"sitting": true, "stand": true, "stood": true, "standing": true, "lose": true,
	"lost": true, "losing": true, "pay": true, "paid": true, "paying": true,
	"meet": true, "met": true, "meeting": true, "include": true, "included": true,
	"including": true, "continue": true, "continued": true, "continuing": true,
	"set": true, "setting": true, "learn": true, "learned": true, "learning": true,
	"change": true, "changed": true, "changing": true, "lead": true, "led": true,
	"leading": true, "understand": true, "understood": true, "understanding": true,
	"watch": true, "watched": true, "watching": true, "follow": true, "followed": true,
	"following": true, "stop": true, "stopped": true, "stopping": true, "create": true,
	"created": true, "creating": true, "speak": true, "spoke": true, "spoken": true,
	"speaking": true, "read": true, "reading": true, "allow": true, "allowed": true,
	"allowing": true, "add": true, "added": true, "adding": true, "spend": true,
	"spent": true, "spending": true, "grow": true, "grew": true, "grown": true,
	"growing": true, "open": true, "opened": true, "opening": true, "walk": true,
	"walked": true, "walking": true, "win": true, "won": true, "winning": true,
	"offer": true, "offered": true, "offering": true, "remember": true, "remembered": true,
	"remembering": true, "love": true, "loved": true, "loving": true, "consider": true,
	"considered": true, "considering": true, "appear": true, "appeared": true,
	"appearing": true, "buy": true, "bought": true, "buying": true, "wait": true,
	"waited": true, "waiting": true, "serve": true, "served": true, "serving": true,
	"die": true, "died": true, "dying": true, "send": true, "sent": true, "sending": true,
	"expect": true, "expected": true, "expecting": true, "build": true, "built": true,
	"building": true, "stay": true, "stayed": true, "staying": true, "fall": true,
	"fell": true, "fallen": true, "falling": true, "cut": true, "cutting": true,
	"reach": true, "reached": true, "reaching": true, "kill": true, "killed": true,
	"killing": true, "remain": true, "remained": true, "remaining": true,
}
