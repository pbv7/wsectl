package commands

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/pbv7/wsectl/internal/worksection"
	"github.com/spf13/cobra"
)

func newFilesCommand(s *state) *cobra.Command {
	cmd := &cobra.Command{Use: "files", Short: "Read and download files"}
	var project, task string
	list := newSimpleActionCommand(s, "list", "get_files", "List files", func(*cobra.Command, []string) map[string]string {
		return map[string]string{"id_project": project, "id_task": task}
	})
	list.Flags().StringVar(&project, "project", "", "Project ID")
	list.Flags().StringVar(&task, "task", "", "Task ID")
	cmd.AddCommand(list)
	var out string
	download := &cobra.Command{
		Use:   "download FILE_ID",
		Short: "Download a file",
		Args:  exactArgsUnlessSchema(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if s.schema {
				return s.runAction(cmd, "download", nil)
			}
			if out == "" {
				return worksection.UsageError("binary download requires --out FILE or --out -")
			}
			clientInfo, err := s.client(cmd.Context(), s.warnSink(cmd))
			if err != nil {
				return err
			}
			s.writeEnvFallbackDiagnostic(cmd, clientInfo)
			resp, err := clientInfo.client.Download(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if out == "-" {
				_, err = cmd.OutOrStdout().Write(resp.Body)
				return err
			}
			return os.WriteFile(out, resp.Body, 0o600)
		},
	}
	download.Flags().StringVar(&out, "out", "", "Destination file, or - for stdout")
	cmd.AddCommand(download)
	cmd.AddCommand(&cobra.Command{
		Use:   "task-attachments TASK_ID",
		Short: "List task attachments",
		Args:  exactArgsUnlessSchema(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if s.schema {
				return s.runAction(cmd, "get_task", nil)
			}
			return s.runAction(cmd, "get_task", map[string]string{"id_task": args[0], "extra": "files"})
		},
	})
	var imageTask, imageProject string
	images := newSimpleActionCommandE(s, "images", "get_files", "List image files", func(*cobra.Command, []string) (map[string]string, error) {
		return map[string]string{"id_task": imageTask, "id_project": imageProject}, nil
	})
	images.RunE = func(cmd *cobra.Command, args []string) error {
		params := map[string]string{"id_task": imageTask, "id_project": imageProject}
		return s.runActionWithOptions(cmd, "get_files", params, false, filterImageFiles)
	}
	images.Flags().StringVar(&imageTask, "task", "", "Task ID")
	images.Flags().StringVar(&imageProject, "project", "", "Project ID")
	cmd.AddCommand(images)
	return cmd
}

func filterImageFiles(data json.RawMessage) (json.RawMessage, []string, error) {
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err != nil {
		return data, []string{"Image filtering was skipped because file response data was not an array."}, nil
	}
	filtered := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if isImageFile(row) {
			filtered = append(filtered, row)
		}
	}
	raw, err := json.Marshal(filtered)
	if err != nil {
		return nil, nil, err
	}
	return raw, []string{"Image filtering is applied client-side; Worksection does not document a server-side image filter for get_files."}, nil
}

func isImageFile(row map[string]any) bool {
	for _, key := range []string{"type", "mime", "mime_type", "content_type"} {
		if value, ok := row[key].(string); ok {
			lower := strings.ToLower(value)
			if strings.HasPrefix(lower, "image/") || lower == "image" {
				return true
			}
		}
	}
	if name, ok := row["name"].(string); ok {
		lower := strings.ToLower(name)
		for _, ext := range []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg", ".heic", ".heif"} {
			if strings.HasSuffix(lower, ext) {
				return true
			}
		}
	}
	return false
}
