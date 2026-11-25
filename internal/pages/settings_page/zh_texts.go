package settings_page

// The gap texts of the page are Chinese (owner rule 2026-11-17); the flag name stays as typed.
const (
	noReaderGapText = "此处无法读取所有者设置。"
	editOffGapText  = "编辑已关闭：请用 --owner-edit 启动 ot serve 后再修改设置。"
)

// The page title and the as-of line read Chinese with the English beside; asOfFormat takes the
// RFC 3339 time twice.
const (
	pageTitle  = "设置 Settings"
	asOfFormat = "设置截至 %s (Settings as of %s)"
)
