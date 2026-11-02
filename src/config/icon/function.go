package icon

const noIconColor = "NONE"

// InitIcon initializes the icon configuration for the application.
// It sets up different icons based on whether nerd fonts are enabled and configures directory icon colors.
//
// Parameters:
//   - nerdfont: boolean flag to determine if nerd fonts should be used
//     When false, uses simple ASCII characters for icons
//     When true, uses nerd font icons (default behavior)
//   - directoryIconColor: string representing the color for directory icons
//     If empty, defaults to "NONE" (dark yellowish)
//
// The function configures various icons for:
//   - System directories (Home, Download, Documents, etc.)
//   - File operations (Compress, Extract, Copy, Cut, Delete)
//   - UI elements (Cursor, Browser, Select, etc.)
//   - Status indicators (Error, Warn, Done, InOperation)
//   - Navigation and sorting (Directory, Search, SortAsc, SortDesc)
func InitIcon(nerdfont bool, directoryIconColor string) {
	if nerdfont {
		Space, SuperfileIcon = " ", "\ue6ad"
		Home, Desktop, Download, Documents = "\U000f02dc", "\U000f01c4", "\U000f03d4", "\U000f0219"
		Pictures, Videos, Music, Templates, PublicShare, Trash = "\U000f02e9", "\U000f0381", "♬", "\U000f03e2", "\uf0ac", "\uf1f8"
		CompressFile, ExtractFile, Copy, Cut, Delete = "\U000f05c4", "\U000f06eb", "\U000f018f", "\U000f0190", "\U000f01b4"
		Cursor, Browser, Select = "\uf054", "\U000f0208", "\U000f01bd"
		Error, Warn, Done, InOperation = "\uf530", "\uf071", "\uf4a4", "\U000f0954"
		Directory, Search, SortAsc, SortDesc, Terminal, Pinned, Disk = "\uf07b", "\ue68f", "\uf0de", "\uf0dd", "\ue795", "\U000f0403", "\U000f11f0"
	}
	// Make sure that these alternatives are ASCII characters only.
	// Dont place any special unicode characters here.
	if !nerdfont {
		// When nerdfont is disabled, we use simple ASCII characters
		// Space is set to empty string because we don't need special spacing
		// for ASCII characters, unlike nerd fonts which often need proper spacing
		// to display correctly
		Space = ""
		SuperfileIcon = ""

		Home = ""
		Desktop = ""
		Trash = ""
		Download = ""
		Documents = ""
		Pictures = ""
		Videos = ""
		Music = ""
		Templates = ""
		PublicShare = ""

		// file operations
		CompressFile = ""
		ExtractFile = ""
		Copy = ""
		Cut = ""
		Delete = ""

		// other
		Cursor = ">"
		Browser = "B"
		Select = "S"
		Error = ""
		Warn = ""
		Done = ""
		InOperation = ""
		Directory = ""
		Search = ""
		SortAsc = "^"
		SortDesc = "v"
		Terminal = ""
		Pinned = ""
		Disk = ""
	}

	if directoryIconColor == "" {
		directoryIconColor = noIconColor // Dark yellowish
	}
	Folders["folder"] = Style{
		Icon:  "\uf07b", // Printable Rune : ""
		Color: directoryIconColor,
	}
}

func GetCopyOrCutIcon(cut bool) string {
	if cut {
		return Cut
	}
	return Copy
}
