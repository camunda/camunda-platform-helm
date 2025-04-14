# Search Package

This package provides functionality for analyzing key usage in Helm templates, with a focus on finding unused values.

## Package Structure

The package is organized into several files to maintain clean separation of concerns:

- **finder.go**: Core structure definitions and high-level methods
- **searcher.go**: File search and pattern matching implementation
- **pattern_checker.go**: Pattern-specific methods for checking key usage with patterns
- **analyzer.go**: Parallel processing and analysis of key usage

## Functionality

### Core Components

- **KeyUsage**: Represents the analysis result for a key
- **Finder**: The main struct that manages searching and analysis

### Key Features

1. **Pattern-based search**: Detect values used through various Helm patterns
2. **Parallel processing**: Distribute work across multiple worker goroutines
3. **Progress tracking**: Visual progress indicators for long-running operations

## Design Decisions

### Parallel Processing

The analysis is performed in three phases:
1. Initial key scanning (parallelized)
2. Complex pattern checking (parallelized)
3. Parent key detection (sequential due to dependencies)

### Search Implementation

Three search strategies are available, with automatic fallback:
1. Direct command execution (ripgrep or grep)
2. Shell-based execution
3. Direct file content search (fallback for problematic patterns)

### Pattern Handling

Custom pattern definitions are used to detect values used in various ways:
- toYaml function calls
- Helm include directives
- Context objects
- Image parameters

## Usage

The primary method to use is `FindUnusedKeys`, which analyzes a list of keys 
and returns information about their usage status in the templates. 