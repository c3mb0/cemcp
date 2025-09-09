# Build Environment Setup for Filesystem Service Tests

## Overview
The filesystem service tests require proper CGO configuration to link with Tesseract and Leptonica libraries for OCR functionality.

## macOS (Homebrew) Setup

### Prerequisites
Ensure Tesseract and Leptonica are installed via Homebrew:
```bash
brew install tesseract leptonica
```

### Build Environment Configuration
Set the following environment variables before running tests:

```bash
export CGO_CPPFLAGS="-I/opt/homebrew/include"
export CGO_LDFLAGS="-L/opt/homebrew/lib -lleptonica -ltesseract"
```

### Running Tests
Navigate to the filesystem service directory and run tests with the configured environment:

```bash
cd services/filesystem
export CGO_CPPFLAGS="-I/opt/homebrew/include"
export CGO_LDFLAGS="-L/opt/homebrew/lib -lleptonica -ltesseract"
go test ./...
```

## Test Results
With proper environment configuration, all tests should pass:
- **84 base tests** plus **fuzz test seeds**: All passing
- **Test duration**: ~15 seconds
- **No compilation errors**: Clean CGO compilation
- **No runtime errors**: All functionality working correctly

## Troubleshooting

### CGO Compilation Errors
If you see errors like "tesseract/baseapi.h: No such file or directory":
1. Verify Tesseract is installed: `brew list tesseract`
2. Check header path exists: `ls /opt/homebrew/include/tesseract/`
3. Ensure CGO_CPPFLAGS includes the correct include path

### Linker Warnings
The warning "ignoring duplicate libraries: '-lleptonica', '-ltesseract'" is expected and harmless. It occurs because some dependencies may also link these libraries.

## Alternative Configurations

For non-Homebrew installations, adjust paths accordingly:
- **MacPorts**: Use `/opt/local/include` and `/opt/local/lib`
- **Manual builds**: Point to your custom installation paths
- **pkg-config**: Use `pkg-config --cflags tesseract lept` and `pkg-config --libs tesseract lept`

## Continuous Integration
For CI environments, ensure the build environment is configured before test execution:
```bash
#!/bin/bash
export CGO_CPPFLAGS="-I/opt/homebrew/include"
export CGO_LDFLAGS="-L/opt/homebrew/lib -lleptonica -ltesseract"
cd services/filesystem
go test ./...
```