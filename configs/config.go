package configs

var DOWNLOAD_DIRECTORY = "/media/runa/NAS/Downloads"
var TEMP_DIRECTORY = "/media/runa/NAS/Downloads/.temp"

// var DOWNLOAD_DIRECTORY = "/downloads"
// var TEMP_DIRECTORY = "/downloads/.temp"
var TEMP_EXT = ".tmpDownload"
var SEG_EXT = ".seg"
var BUFF_SIZE = 1024 * 1024 // 1 MB
var FILE_BUFF_SIZE = 1024 * 1024
var DEFAULT_USER_AGENT = "Mozilla/5.0 (Macintosh; Intel Mac OS X x.y; rv:10.0) Gecko/20100101 Firefox/10.0"
var SEGMENT_SIZE int64 = 1024 * 1024 * 5
var BANDWIDTH float64 = 1024 * 1024 * 50

var VIDEO_SUB_FOLDER = "Video"
var PROGRAMS_SUB_FOLDER = "Program"
var MUSIC_SUB_FOLDER = "Music"
var DOCUMENT_SUB_FOLDER = "Document"
var COMPRESSED_SUB_FOLDER = "Compressed"
var GENERAL_SUB_FOLDER = "General"
var VideoExtensions = []string{
	".mp4",
	".avi",
	".mov",
	".mkv",
	".wmv",
	".flv",
	".webm",
	".mpeg",
	".mpg",
	".3gp",
	".m4v",
	".ts",
}
var ProgramExtensions = []string{
	".exe", // Windows Executable
	".msi", // Windows Installer
	".apk", // Android Package
	".dmg", // macOS Disk Image
	".deb", // Debian Package
	".rpm", // Red Hat Package
	".bin", // Binary file
	".jar", // Java Archive
	".py",  // Python Script
	".sh",  // Shell Script
	".bat", // Batch file
}

var CompressedExtensions = []string{
	".zip",
	".tar",
	".gz",
	".bz2",
	".7z",
	".rar",
	".xz",
	".tgz",
	".tbz",
	".zipx",
}

var DocumentExtensions = []string{
	".pdf",
	".doc",
	".docx",
	".ppt",
	".pptx",
	".xls",
	".xlsx",
	".txt",
	".csv",
	".rtf",
	".md",
}

var MusicExtensions = []string{
	".mp3",
	".wav",
	".aac",
	".ogg",
	".flac",
	".m4a",
	".wma",
	".aiff",
	".opus",
	".mid",
}
