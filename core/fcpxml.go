package core

import (
	"encoding/xml"
	"strings"
	"fmt"      
	"os"       
    	"path"    
	"path/filepath" 

	"github.com/asticode/go-astisub"
	"github.com/hnlq715/srt2fcpxml/core/FcpXML"
	"github.com/hnlq715/srt2fcpxml/core/FcpXML/Library"
	"github.com/hnlq715/srt2fcpxml/core/FcpXML/Library/Event"
	"github.com/hnlq715/srt2fcpxml/core/FcpXML/Library/Event/Project"
	"github.com/hnlq715/srt2fcpxml/core/FcpXML/Library/Event/Project/Sequence"
	"github.com/hnlq715/srt2fcpxml/core/FcpXML/Library/Event/Project/Sequence/Spine"
	"github.com/hnlq715/srt2fcpxml/core/FcpXML/Library/Event/Project/Sequence/Spine/Gap"
	"github.com/hnlq715/srt2fcpxml/core/FcpXML/Library/Event/Project/Sequence/Spine/Gap/Title"
	"github.com/hnlq715/srt2fcpxml/core/FcpXML/Resources"
)

// getPhysicalPath translates FCP virtual path to the real filesystem path
func getPhysicalPath(motiPath string) string {
	// 1. Get the current user's home directory dynamically
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to a safe default if home cannot be determined
		home = "/Users" 
	}

	// 2. Handle sudo environment: redirect from /var/root back to the real user
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		home = "/Users/" + sudoUser
	}

	// 3. Define the mandatory FCP template directory structure on macOS
	// We use "Movies/Motion Templates.localized" as the bridge
	fcpRelativeRoot := "Movies/Motion Templates.localized"

	// 4. Translate "~/..." to "/Users/username/Movies/Motion Templates.localized/..."
	// Remove the "~/" prefix from the input and join it with our resolved physical root
	relativePath := strings.TrimPrefix(motiPath, "~/")
	return filepath.Join(home, fcpRelativeRoot, relativePath)
}

func Srt2FcpXmlExport(projectName string, frameDuration interface{}, subtitles *astisub.Subtitles, width, height int, moti_path string) ([]byte, error) {
	fcpxml := FcpXML.New()
	res := Resources.NewResources()
	effect := Resources.NewEffect()

	// Process custom Motion template path if provided
	if moti_path != "" {
		// Strict validation: Path MUST start with "~/Titles.localized" for FCP compatibility
		if strings.HasPrefix(moti_path, "~/Titles.localized") {
			// --- Use the helper function to get the actual path for os.Stat ---
			checkPath := getPhysicalPath(moti_path)
			if _, err := os.Stat(checkPath); os.IsNotExist(err) {
				fmt.Printf("Warning: Motion template file not found at: %s\nUsing basic title instead.\n", checkPath)
			} else {
				// Validation passed: Set Uid using the FCP virtual format (~/)
				effect.Uid = moti_path
				
				// Extract filename for the effect name resource
				baseName := path.Base(moti_path)
				effect.Name = strings.TrimSuffix(baseName, path.Ext(baseName))
			}
		} else {
			// Path doesn't start with the required prefix
			fmt.Printf("Warning: Path %s must start with '~/Titles.localized'. Using basic title.\n", moti_path)
		}
	}
	res.SetEffect(effect)
	format := Resources.NewFormat().
		SetWidth(width).
		SetHeight(height).
		SetFrameRate(frameDuration).Render()
	res.SetFormat(format)
	fcpxml.SetResources(res)
	gap := Gap.NewGap(subtitles.Duration().Seconds())

	for index, item := range subtitles.Items {
		textStyleDef := Title.NewTextStyleDef(index + 1)
		text := Title.NewContent(index+1, func(lines []astisub.Line) string {
			var os []string
			for _, l := range lines {
				os = append(os, l.String())
			}
			return strings.Trim(strings.Join(os, "\n"), "\n")
		}(item.Lines))
		title := Title.NewTitle(item.String(), item.StartAt.Seconds(), item.EndAt.Seconds()).SetTextStyleDef(textStyleDef).SetText(text)
		title.AddParam(Title.NewParams("Position", "9999/999166631/999166633/1/100/101", "0 -450"))
		title.AddParam(Title.NewParams("Alignment", "9999/999166631/999166633/2/354/999169573/401", "1 (Center)"))
		title.AddParam(Title.NewParams("Flatten", "9999/999166631/999166633/2/351", "1"))
		title.AddParam(Title.NewParams("Build In", "9999/10000/2/101", "0"))
		title.AddParam(Title.NewParams("Build Out", "9999/10000/2/102", "0"))
		title.AddParam(Title.NewParams("ScaleY", "9999/1825768479/100/1825768480/2/100", "0"))
		title.AddParam(Title.NewParams("Opacity", "9999/1825768325/10003/10045/1/200/202", "0.7"))
		gap.AddTitle(title)
	}

	spine := Spine.NewSpine().SetGap(gap)
	seq := Sequence.NewSequence(subtitles.Duration().Seconds()).SetSpine(spine)
	project := Project.NewProject(projectName).SetSequence(seq)
	event := Event.NewEvent().SetProject(project)
	library := Library.NewLibrary(projectName).SetEvent(event)
	fcpxml.SetLibrary(library)

	return xml.MarshalIndent(fcpxml, "", "    ")
}
