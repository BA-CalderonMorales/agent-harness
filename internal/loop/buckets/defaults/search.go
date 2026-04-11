package defaults

// Search bucket defaults
const (
	// Result limits
	SearchMaxResultsDefault   = 50
	SearchMaxResultsFast      = 20
	SearchMaxResultsRobust    = 100
	SearchMaxResultsUnlimited = 1000

	// Line limits
	SearchMaxLineLength  = 500
	SearchMaxLineDefault = 200

	// Context lines
	SearchContextLinesDefault = 2
	SearchContextLinesNone    = 0
	SearchContextLinesMore    = 5

	// File patterns
	SearchDefaultFilePattern = "*"
)

// SearchExcludedDirs contains directories to exclude from searches
var SearchExcludedDirs = []string{
	".git",
	".svn",
	".hg",
	"node_modules",
	"vendor",
	".venv",
	"venv",
	"__pycache__",
	".pytest_cache",
	"target",
	"build",
	"dist",
	".next",
	".nuxt",
	".cache",
	".idea",
	".vscode",
}

// SearchExcludedFiles contains file patterns to exclude
var SearchExcludedFiles = []string{
	"*.min.js",
	"*.min.css",
	"*.map",
	"*.lock",
	"package-lock.json",
	"yarn.lock",
	"pnpm-lock.yaml",
	"Cargo.lock",
	"*.wasm",
	"*.dylib",
	"*.so",
	"*.dll",
	"*.exe",
	"*.bin",
}

// SearchBinaryExtensions contains file extensions that are binary
var SearchBinaryExtensions = []string{
	".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg",
	".mp3", ".mp4", ".avi", ".mov", ".webm",
	".pdf", ".doc", ".docx", ".xls", ".xlsx",
	".zip", ".tar", ".gz", ".bz2", ".7z", ".rar",
	".exe", ".dll", ".so", ".dylib",
	".wasm", ".o", ".a",
}

// SearchCodeExtensions contains source code extensions
var SearchCodeExtensions = []string{
	".go", ".js", ".ts", ".jsx", ".tsx",
	".py", ".rb", ".rs", ".java", ".kt", ".scala",
	".c", ".cpp", ".h", ".hpp", ".cc", ".cxx",
	".php", ".swift", ".m", ".mm",
	".r", ".pl", ".pm", ".lua",
}
