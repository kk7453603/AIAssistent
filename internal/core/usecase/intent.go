package usecase

import "strings"

// Intent represents the classified category of a user request.
type Intent string

const (
	IntentKnowledge Intent = "knowledge"
	IntentCode      Intent = "code"
	IntentFile      Intent = "file"
	IntentTask      Intent = "task"
	IntentWeb       Intent = "web"
	IntentGeneral   Intent = "general"
)

// intentPriority defines a deterministic evaluation order for intents,
// preventing non-deterministic map iteration from affecting classification.
var intentPriority = []Intent{
	IntentCode,
	IntentFile,
	IntentTask,
	IntentWeb,
}

var intentKeywords = map[Intent][]string{
	IntentCode: {
		"python", "код", "скрипт", "выполни", "execute", "bash",
		"запусти", "вычисли", "calculate", "compute", "execute_python",
		"execute_bash", "напиши код", "run code",
	},
	IntentFile: {
		"файл", "прочитай", "каталог", "директори", "file", "read_file",
		"list_directory", "directory", "папк", "содержимое файла", "открой файл",
	},
	IntentTask: {
		"задач", "task", "напомни", "todo", "дело",
	},
	IntentWeb: {
		"интернет", "web", "найди в сети", "загугли", "поищи онлайн",
		"search online", "найди в интернете",
	},
}

// classifyIntentByKeywords uses keyword matching to determine intent.
// Returns IntentGeneral if no keywords match.
// Evaluation follows intentPriority order to ensure deterministic results.
func classifyIntentByKeywords(message string) Intent {
	lower := strings.ToLower(message)
	for _, intent := range intentPriority {
		for _, kw := range intentKeywords[intent] {
			if strings.Contains(lower, kw) {
				return intent
			}
		}
	}
	return IntentGeneral
}

// systemPromptForIntent returns an instruction string that guides the LLM
// to use the appropriate tools based on the detected intent.
func systemPromptForIntent(intent Intent) string {
	switch intent {
	case IntentCode:
		return "The user wants to execute code or do calculations. Use execute_python or execute_bash tools directly."
	case IntentFile:
		return "The user wants to work with files. Use filesystem tools (read_file, read_text_file, list_directory, search_files) directly."
	case IntentTask:
		return "The user wants to manage tasks. Use task_create, task_list, task_update, task_complete, or task_delete."
	case IntentWeb:
		return "The user needs internet search. Use web_search tool."
	case IntentKnowledge:
		return "Search the knowledge base first using knowledge_search."
	default: // IntentGeneral
		return "Answer from your knowledge. If unsure, search the knowledge base first using knowledge_search."
	}
}
