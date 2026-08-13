package common

import "fmt"

// WrapBilingualContent wraps email content with both English and Chinese versions.
// English content is placed at the top, followed by Chinese content.
// A horizontal rule separates the two sections.
func WrapBilingualContent(enContent string, zhContent string) string {
	return fmt.Sprintf(
		`%s`+
			`<hr style="border:none;border-top:1px solid #ddd;margin:20px 0;">`+
			`%s`,
		enContent, zhContent,
	)
}

// WrapBilingualSubject returns a bilingual email subject with English first.
func WrapBilingualSubject(enSubject string, zhSubject string) string {
	return fmt.Sprintf("%s / %s", enSubject, zhSubject)
}
