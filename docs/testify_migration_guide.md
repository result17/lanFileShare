# Testify Migration Guide

本文档提供了将原生 Go 测试断言迁移到 testify 的详细指南。

## 快速参考

### 常用断言替换对照表

| 原生断言                            | Testify 替换                        | 说明         |
| ----------------------------------- | ----------------------------------- | ------------ |
| `if err != nil { t.Fatalf("...") }` | `require.NoError(t, err, "...")`    | 致命错误检查 |
| `if err == nil { t.Fatal("...") }`  | `require.Error(t, err, "...")`      | 期望错误检查 |
| `if x != y { t.Errorf("...") }`     | `assert.Equal(t, y, x, "...")`      | 相等性检查   |
| `if x == nil { t.Error("...") }`    | `assert.NotNil(t, x, "...")`        | 非空检查     |
| `if len(x) == 0 { t.Error("...") }` | `assert.NotEmpty(t, x, "...")`      | 非空集合检查 |
| `if !condition { t.Error("...") }`  | `assert.True(t, condition, "...")`  | 布尔检查     |
| `if condition { t.Error("...") }`   | `assert.False(t, condition, "...")` | 布尔检查     |

### 文件和目录断言

```go
// 文件存在检查
assert.FileExists(t, "/path/to/file", "File should exist")
assert.NoFileExists(t, "/path/to/file", "File should not exist")

// 目录存在检查
assert.DirExists(t, "/path/to/dir", "Directory should exist")
```

### 字符串断言

```go
// 包含检查
assert.Contains(t, haystack, needle, "Should contain substring")
assert.NotContains(t, haystack, needle, "Should not contain substring")

// 前缀/后缀检查
assert.True(t, strings.HasPrefix(str, prefix), "Should have prefix")
```

### 集合断言

```go
// 长度检查
assert.Len(t, slice, expectedLen, "Should have correct length")

// 元素检查
assert.Contains(t, slice, element, "Should contain element")
assert.ElementsMatch(t, expected, actual, "Should contain same elements")
```

## 迁移步骤

### 1. 添加 testify 导入

```go
import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)
```

### 2. 替换基本断言

**之前：**

```go
func TestExample(t *testing.T) {
    result, err := someFunction()
    if err != nil {
        t.Fatalf("Function failed: %v", err)
    }

    if result != "expected" {
        t.Errorf("Expected 'expected', got '%s'", result)
    }
}
```

**之后：**

```go
func TestExample(t *testing.T) {
    result, err := someFunction()
    require.NoError(t, err, "Function should succeed")

    assert.Equal(t, "expected", result, "Result should match expected value")
}
```

### 3. 处理复杂条件

**之前：**

```go
if data == nil {
    t.Error("Data should not be nil")
} else if len(data.Items) == 0 {
    t.Error("Data should have items")
} else if data.Items[0].Value != "test" {
    t.Errorf("Expected 'test', got '%s'", data.Items[0].Value)
}
```

**之后：**

```go
require.NotNil(t, data, "Data should not be nil")
assert.NotEmpty(t, data.Items, "Data should have items")
assert.Equal(t, "test", data.Items[0].Value, "First item should have correct value")
```

## 最佳实践

### 1. 使用 require vs assert

- **require**: 用于关键检查，失败时停止测试
- **assert**: 用于业务逻辑验证，失败时继续测试

```go
// 使用 require 进行设置检查
tempDir, err := os.MkdirTemp("", "test")
require.NoError(t, err, "Failed to create temp directory")

// 使用 assert 进行业务逻辑检查
result := processData(tempDir)
assert.Equal(t, expectedResult, result, "Processing should return expected result")
assert.True(t, result.IsValid, "Result should be valid")
```

### 2. 提供有意义的消息

```go
// 好的做法
assert.Equal(t, expectedCount, len(items), "Should have correct number of items after filtering")

// 避免的做法
assert.Equal(t, expectedCount, len(items))
```

### 3. 使用子测试组织相关测试

```go
func TestFileProcessor(t *testing.T) {
    processor := NewFileProcessor()

    t.Run("valid_file", func(t *testing.T) {
        result, err := processor.Process("valid.txt")
        require.NoError(t, err, "Should process valid file")
        assert.NotNil(t, result, "Should return result")
    })

    t.Run("invalid_file", func(t *testing.T) {
        result, err := processor.Process("invalid.txt")
        require.Error(t, err, "Should fail with invalid file")
        assert.Nil(t, result, "Should not return result on error")
    })
}
```

## 常见模式

### 错误处理模式

```go
// 期望成功
result, err := operation()
require.NoError(t, err, "Operation should succeed")
assert.NotNil(t, result, "Should return valid result")

// 期望失败
result, err := invalidOperation()
require.Error(t, err, "Operation should fail")
assert.Contains(t, err.Error(), "expected error message", "Should have meaningful error")
assert.Nil(t, result, "Should not return result on error")
```

### 文件操作模式

```go
// 创建测试文件
tempDir, err := os.MkdirTemp("", "test")
require.NoError(t, err, "Failed to create temp directory")
defer os.RemoveAll(tempDir)

testFile := filepath.Join(tempDir, "test.txt")
err = os.WriteFile(testFile, []byte("test content"), 0644)
require.NoError(t, err, "Failed to create test file")

// 验证文件操作
assert.FileExists(t, testFile, "Test file should exist")

content, err := os.ReadFile(testFile)
require.NoError(t, err, "Should read test file")
assert.Equal(t, "test content", string(content), "File content should match")
```

### HTTP 测试模式

```go
// 创建测试服务器
server := httptest.NewServer(handler)
defer server.Close()

// 发送请求
resp, err := http.Get(server.URL + "/api/test")
require.NoError(t, err, "Request should succeed")
defer resp.Body.Close()

// 验证响应
assert.Equal(t, http.StatusOK, resp.StatusCode, "Should return OK status")

body, err := io.ReadAll(resp.Body)
require.NoError(t, err, "Should read response body")
assert.Contains(t, string(body), "expected", "Response should contain expected content")
```

## 迁移检查清单

- [ ] 添加 testify 导入
- [ ] 替换所有 `t.Fatal*` 为 `require.*`
- [ ] 替换所有 `t.Error*` 为 `assert.*`
- [ ] 使用专用断言（如 `FileExists`, `Contains` 等）
- [ ] 为所有断言添加有意义的消息
- [ ] 运行测试确保迁移成功
- [ ] 检查测试覆盖率没有下降

## 工具和脚本

可以使用以下命令查找需要迁移的断言：

```bash
# 查找原生断言
grep -r "t\.Fatal\|t\.Error" --include="*_test.go" .

# 查找缺少 testify 导入的测试文件
grep -L "stretchr/testify" --include="*_test.go" -r .
```
