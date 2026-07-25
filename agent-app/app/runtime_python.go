package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/kageos/kageos-sdk/agent-app/response"
	pythonRuntime "github.com/kageos/kageos-sdk/agent-app/runtime/python"
	appTypes "github.com/kageos/kageos-sdk/agent-app/types"
	"github.com/kageos/kageos-sdk/dto"
	"github.com/kageos/kageos-sdk/pkg/logger"
)

const runtimePythonRouter = "/_runtime/python"

// runtimePythonArgs 兼容内部 JSON 对象和历史 text_area 字符串对象。
type runtimePythonArgs string

func (a *runtimePythonArgs) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*a = ""
		return nil
	}

	if data[0] == '"' {
		var raw string
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
		raw = strings.TrimSpace(raw)
		if raw == "" {
			*a = ""
			return nil
		}
		data = []byte(raw)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("args 必须是 JSON 对象或 JSON 对象字符串: %w", err)
	}
	if parsed == nil {
		parsed = make(map[string]interface{})
	}
	*a = runtimePythonArgs(string(data))
	return nil
}

func (a runtimePythonArgs) Map() map[string]interface{} {
	raw := strings.TrimSpace(string(a))
	if raw == "" {
		return map[string]interface{}{}
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil || parsed == nil {
		return map[string]interface{}{}
	}
	return parsed
}

type runtimePythonReq struct {
	PythonCode         string            `json:"python_code" validate:"required"`
	Args               runtimePythonArgs `json:"args"`
	InputFiles         string            `json:"input_files"`
	Packages           string            `json:"packages"`
	TimeoutSeconds     int               `json:"timeout_seconds"`
	CollectOutputFiles bool              `json:"collect_output_files"`
}

// RuntimePython 执行 SDK 默认私有 Python runtime 路由。
//
// 该路由由平台内部调用，不进入 schema / service tree；对外协议由 agent-server
// 的 run_python 工具保持稳定。
func (a *App) RuntimePython(ctx *Context, resp response.Response) error {
	var req runtimePythonReq
	if err := ctx.ShouldBindValidate(&req); err != nil {
		return err
	}

	requestData := make(map[string]interface{})
	for k, v := range req.Args.Map() {
		requestData[k] = v
	}

	if inputFileRefs := runtimePythonInputFileRefs(req.InputFiles, requestData); inputFileRefs != "" {
		fs := ctx.GetFS()
		downloadResult := fs.DownloadFilesDetailed(inputFileRefs)
		downloadedInputFiles := downloadResult.Paths
		defer fs.RemoveFiles(downloadedInputFiles)
		if len(downloadedInputFiles) == 0 {
			detail := downloadResult.ErrorMessage()
			if detail != "" {
				detail = "原因：" + detail + "。"
			}
			return resp.Form(&dto.RunPythonRuntimeResp{
				Output:     "输入文件下载失败：未能把文件引用下载到容器本地路径。" + detail + "请确认 input_files 为 bucket/object_key 字符串，且文件已上传完成。",
				Status:     "失败",
				JSONResult: "（无结构化结果）",
			}).Build()
		}
		requestData["input_files"] = downloadedInputFiles
	}

	timeout := 5 * time.Minute
	if req.TimeoutSeconds > 0 {
		timeout = time.Duration(req.TimeoutSeconds) * time.Second
	}

	outputDir := ctx.GetFS().GetTraceOutputDir()
	executor := pythonRuntime.NewExecutor(runtimeNormalizePythonCode(req.PythonCode)).
		WithRequest(requestData).
		WithOutputDir(outputDir).
		WithTimeout(timeout)
	defer func() {
		if closeErr := executor.Close(); closeErr != nil {
			logger.Warnf(ctx, "[RuntimePython] executor.Close: %v", closeErr)
		}
	}()

	if packages := runtimeSplitPythonPackages(req.Packages); len(packages) > 0 {
		executor = executor.WithPackages(packages...)
	}

	execResult, err := executor.ExecuteResult(ctx)
	out := runtimeBuildPythonResponse(ctx, req, execResult, err)
	return resp.Form(out).Build()
}

func runtimeBuildPythonResponse(ctx *Context, req runtimePythonReq, execResult *pythonRuntime.ExecutionResult, err error) *dto.RunPythonRuntimeResp {
	outputStr := ""
	status := "成功"
	jsonResult := ""
	outputFiles := ""

	if execResult != nil {
		outputStr = execResult.CombinedOutput()
		jsonResult = execResult.FormattedData()
	}

	if err != nil {
		logger.Errorf(ctx, "[RuntimePython] 执行 Python 代码失败: %v, output: %s", err, outputStr)
		status = "失败"
		if strings.TrimSpace(outputStr) != "" {
			outputStr = fmt.Sprintf("执行错误: %v\n\n输出:\n%s", err, outputStr)
		} else {
			outputStr = fmt.Sprintf("执行错误: %v", err)
		}
	} else if execResult != nil {
		logger.Debugf(ctx, "[RuntimePython] 执行 Python 代码成功")
		if req.CollectOutputFiles || len(execResult.OutputFiles) > 0 {
			paths, pathErr := execResult.OutputFilePaths()
			if pathErr != nil {
				logger.Errorf(ctx, "[RuntimePython] 校验 output_files 失败: %v", pathErr)
				status = "失败"
				if outputStr != "" {
					outputStr += "\n\n输出文件错误:\n" + pathErr.Error()
				} else {
					outputStr = "输出文件错误: " + pathErr.Error()
				}
			} else if len(paths) > 0 {
				responseFiles, nameErr := runtimePythonResponseFiles(paths, execResult.OutputFiles)
				if nameErr != nil {
					logger.Errorf(ctx, "[RuntimePython] 校验 output_files.name 失败: %v", nameErr)
					status = "失败"
					outputStr = appendRuntimeOutputFileError(outputStr, nameErr.Error())
				} else {
					outputFiles = ctx.GetFS().ResponseFilesWithNames(responseFiles)
					if uploadErr := runtimeValidateUploadedFileCount(outputFiles, len(responseFiles)); uploadErr != nil {
						logger.Errorf(ctx, "[RuntimePython] 上传 output_files 不完整: %v", uploadErr)
						status = "失败"
						outputStr = appendRuntimeOutputFileError(outputStr, uploadErr.Error())
					}
				}
			}
		}
	}

	if outputStr == "" {
		outputStr = "（脚本执行完成，无输出）"
	}
	if jsonResult == "" {
		jsonResult = "（无结构化结果）"
	}

	return &dto.RunPythonRuntimeResp{
		Output:      outputStr,
		Status:      status,
		JSONResult:  jsonResult,
		OutputFiles: outputFiles,
	}
}

func runtimePythonResponseFiles(paths []string, artifacts []pythonRuntime.PythonArtifact) ([]ResponseFile, error) {
	files := make([]ResponseFile, 0, len(paths))
	for i, filePath := range paths {
		name := ""
		if i < len(artifacts) {
			rawName := strings.TrimSpace(artifacts[i].Name)
			if rawName != "" {
				name = normalizeResponseFileName(rawName)
				if name == "" {
					return nil, fmt.Errorf("python output_files[%d].name 不是有效文件名", i)
				}
				pathExt := strings.ToLower(filepath.Ext(filePath))
				nameExt := strings.ToLower(filepath.Ext(name))
				if pathExt != nameExt {
					return nil, fmt.Errorf("python output_files[%d].name 扩展名必须与 path 一致: path=%s, name=%s", i, filepath.Base(filePath), name)
				}
			}
		}
		files = append(files, ResponseFile{Path: filePath, Name: name})
	}
	return files, nil
}

func runtimeValidateUploadedFileCount(outputFiles string, expected int) error {
	actual := len(appTypes.ParseFileRefs(outputFiles))
	if actual != expected {
		return fmt.Errorf("输出文件上传不完整：应上传 %d 个，实际成功 %d 个", expected, actual)
	}
	return nil
}

func appendRuntimeOutputFileError(output, detail string) string {
	detail = strings.TrimSpace(detail)
	if strings.TrimSpace(output) == "" {
		return "输出文件错误: " + detail
	}
	return output + "\n\n输出文件错误:\n" + detail
}

func runtimePythonInputFileRefs(explicit string, args map[string]interface{}) string {
	if s := strings.TrimSpace(explicit); s != "" {
		return s
	}
	for _, key := range []string{"input_files", "files", "refs"} {
		if refs := runtimeNormalizeInputFileRefsValue(args[key]); refs != "" {
			return refs
		}
	}
	return ""
}

func runtimeNormalizeInputFileRefsValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case []string:
		return strings.Join(runtimeCleanStringParts(v), ",")
	case []interface{}:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(runtimeCleanStringParts(parts), ",")
	case map[string]interface{}:
		return runtimeNormalizeInputFileRefsValue(v["refs"])
	default:
		return ""
	}
}

func runtimeSplitPythonPackages(packagesStr string) []string {
	return runtimeCleanStringParts(strings.Split(packagesStr, ","))
}

func runtimeCleanStringParts(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func runtimeNormalizePythonCode(code string) string {
	return strings.TrimPrefix(code, "\ufeff")
}
