package common

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"

	"github.com/yorukot/superfile/src/config/icon"
)

func getFileIcon(file string, isLink bool) icon.Style {
	if isLink {
		return icon.Icons["link_file"]
	}
	ext := strings.TrimPrefix(filepath.Ext(file), ".")
	// default icon for all files. try to find a better one though...
	resultIcon := icon.Icons["file"]
	// resolve aliased extensions
	extKey := strings.ToLower(ext)
	alias, hasAlias := icon.Aliases[extKey]
	if hasAlias {
		extKey = alias
	}

	// see if we can find a better icon based on extension alone
	betterIcon, hasBetterIcon := icon.Icons[extKey]
	if hasBetterIcon {
		resultIcon = betterIcon
	}

	// now look for icons based on full names
	fullName := file

	fullName = strings.ToLower(fullName)
	fullAlias, hasFullAlias := icon.Aliases[fullName]
	if hasFullAlias {
		fullName = fullAlias
	}
	bestIcon, hasBestIcon := icon.Icons[fullName]
	if hasBestIcon {
		resultIcon = bestIcon
	}
	if resultIcon.Color == "NONE" {
		return icon.Style{
			Icon:  resultIcon.Icon,
			Color: Theme.FilePanelFG,
		}
	}
	return resultIcon
}

func GetElementIcon(file string, isDir bool, isLink bool, nerdFont bool) icon.Style {
	style := getElementIcon(file, isDir, isLink, nerdFont)
	if Config.ColorFileIcons && !Config.PlainMode {
		style.Color = fileTypeIconColor(file, isDir, isLink)
	} else {
		style.Color = Theme.FilePanelFG
	}
	if _, noColor := os.LookupEnv("NO_COLOR"); noColor {
		style.Color = ""
	}
	return style
}

func getElementIcon(file string, isDir bool, isLink bool, nerdFont bool) icon.Style {
	if !nerdFont {
		marker := ""
		if !Config.PlainMode {
			marker = compactFileMarker(file, isDir, isLink)
		}
		return icon.Style{
			Icon:  marker,
			Color: Theme.FilePanelFG,
		}
	}

	if isDir {
		if isLink {
			return icon.Folders["link_folder"]
		}
		resultIcon := icon.Folders["folder"]
		betterIcon, hasBetterIcon := icon.Folders[file]
		if hasBetterIcon {
			resultIcon = betterIcon
		}
		return resultIcon
	}

	return getFileIcon(file, isLink)
}

// Compact markers use monochrome Unicode, not emoji or a private-use icon font.
func compactFileMarker(file string, isDir, isLink bool) string {
	if isLink {
		return "↗"
	}
	if isDir {
		return "▸"
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(file), "."))
	if alias, ok := icon.Aliases[ext]; ok {
		ext = alias
	}
	switch ext {
	case "image":
		return "▧"
	case "zip":
		return "▥"
	case "txt", "md", "markdown", "pdf", "doc", "rtf", "odt", "ebook", "csv", "xls", "ppt":
		return "≡"
	default:
		return "□"
	}
}

func GetDirectoryIcon(path string, name string, nerdFont bool) string {
	if !nerdFont {
		return ""
	}

	switch path {
	case xdg.Home:
		return icon.Home
	case xdg.UserDirs.Desktop:
		return icon.Desktop
	case xdg.UserDirs.Download:
		return icon.Download
	case xdg.UserDirs.Documents:
		return icon.Documents
	case xdg.UserDirs.Pictures:
		return icon.Pictures
	case xdg.UserDirs.Videos:
		return icon.Videos
	case xdg.UserDirs.Music:
		return icon.Music
	case xdg.UserDirs.Templates:
		return icon.Templates
	case xdg.UserDirs.PublicShare:
		return icon.PublicShare
	default:
		result, exists := icon.Folders[name]
		if !exists {
			return icon.Directory
		}

		return result.Icon
	}
}
