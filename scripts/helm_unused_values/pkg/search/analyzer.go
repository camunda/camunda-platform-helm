package search

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/schollz/progressbar/v3"
)

// Determine the number of worker goroutines to use based on configuration
func (f *Finder) getWorkerCount() int {
	if f.Parallelism > 0 {
		// Use the explicitly configured value
		return f.Parallelism
	}

	// Auto-configure based on CPU cores
	numWorkers := runtime.NumCPU()
	if numWorkers < 2 {
		numWorkers = 2 // Minimum 2 workers
	} else if numWorkers > 8 {
		numWorkers = 8 // Cap at 8 workers to avoid too many concurrent processes
	}
	return numWorkers
}

// createProgressBar creates a configured progress bar for the analysis process
func createProgressBar(total int, description string, color string) *progressbar.ProgressBar {
	return progressbar.NewOptions(total,
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(false),
		progressbar.OptionSetWidth(50),
		progressbar.OptionSetDescription(description),
		progressbar.OptionUseANSICodes(true),    // Use ANSI codes for better positioning
		progressbar.OptionSetPredictTime(false), // Don't show ETA
		progressbar.OptionSpinnerType(14),       // Use a dot spinner
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[" + color + "]=[reset]",
			SaucerHead:    "[" + color + "]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}))
}

// analyzeKeys analyzes the keys usage and returns the result
// This is the implementation of the exported FindUnusedKeys method
func (f *Finder) analyzeKeys(keys []string, showProgress bool) ([]KeyUsage, error) {
	// Create a slice to hold all usage data
	usages := make([]KeyUsage, len(keys))

	// Create map to track used keys (using a mutex for thread safety)
	usedKeysMap := make(map[string]bool)
	var usedKeysMutex sync.Mutex

	// Create progress bar
	var bar *progressbar.ProgressBar
	if showProgress {
		bar = createProgressBar(len(keys), "Analyzing keys...", "green")
	}

	// Determine the number of worker goroutines to use
	numWorkers := f.getWorkerCount()

	if f.Debug {
		fmt.Printf("\033[1;36mParallel execution: Using %d worker goroutines\033[0m\n", numWorkers)
	}

	// Create a work distributor to handle parallel processing
	results := f.processKeysInParallel(keys, usages, usedKeysMap, &usedKeysMutex, bar, showProgress, numWorkers)
	if showProgress {
		bar.Finish()
		fmt.Println() // Add newline after progress bar
	}

	if showProgress {
		bar.Finish()
	}

	return results, nil
}

// Type definition for a work item in the parallel processing pipeline
type workItem struct {
	index int
	key   string
}

// processKeysInParallel distributes key analysis across multiple worker goroutines
func (f *Finder) processKeysInParallel(
	keys []string,
	usages []KeyUsage,
	usedKeysMap map[string]bool,
	usedKeysMutex *sync.Mutex,
	bar *progressbar.ProgressBar,
	showProgress bool,
	numWorkers int,
) []KeyUsage {
	// Create channels for work distribution and synchronization
	jobs := make(chan workItem, len(keys))
	var wg sync.WaitGroup

	// Create a shared progressUpdate channel to update progress bar from workers
	progressUpdates := make(chan int, len(keys))

	// Start progress updater in a separate goroutine
	if showProgress {
		go func() {
			for range progressUpdates {
				bar.Add(1)
			}
		}()
	}

	// Start worker goroutines
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for job := range jobs {
				// Skip empty keys
				if job.key == "" {
					if showProgress {
						progressUpdates <- 1
					}
					continue
				}

				// Process key
				usage := KeyUsage{
					Key:       job.key,
					IsUsed:    false,
					UsageType: "unused",
				}

				// Check if the key is directly used in templates
				isDirectlyUsed, locations := f.searchForDirectUsageOfKeyAcrossAllTemplates(job.key)
				if isDirectlyUsed {
					usage.IsUsed = true
					usage.UsageType = "direct"
					usage.Locations = locations
					usedKeysMutex.Lock()
					usedKeysMap[job.key] = true
					usedKeysMutex.Unlock()
					usages[job.index] = usage
					if showProgress {
						progressUpdates <- 1
					}
					continue
				}

				// Check if key is used by any registered patterns
				for _, patternName := range f.Registry.Names {
					// Check if key is used with this pattern
					isUsed, parent, files := f.isKeyUsedWithPattern(job.key, patternName)
					if isUsed {
						usage.IsUsed = true
						usage.UsageType = "pattern"
						usage.Locations = files
						usage.ParentKey = parent
						usage.PatternName = patternName
						usedKeysMutex.Lock()
						usedKeysMap[job.key] = true
						usedKeysMutex.Unlock()
						break
					}
				}

				usages[job.index] = usage
				if showProgress {
					progressUpdates <- 1
				}
			}
		}(w)
	}

	// Send jobs to workers
	for i, key := range keys {
		if showProgress && i%10 == 0 {
			bar.Describe(fmt.Sprintf("Analyzing key: %s", key))
		}
		jobs <- workItem{index: i, key: key}
	}

	// Close jobs channel to signal workers to exit
	close(jobs)

	// Wait for all workers to finish
	wg.Wait()

	// Close progress updates channel
	if showProgress {
		close(progressUpdates)
	}

	return usages
}

// processComplexPatterns performs a second pass analysis for complex pattern usage
func (f *Finder) processComplexPatterns(
	usages []KeyUsage,
	usedKeysMap map[string]bool,
	usedKeysMutex *sync.Mutex,
	bar *progressbar.ProgressBar,
	showProgress bool,
	numWorkers int,
) []KeyUsage {
	// Create channels for work distribution and synchronization
	jobs := make(chan workItem, len(usages))
	var wg sync.WaitGroup

	// Create a shared progressUpdate channel to update progress bar from workers
	progressUpdates := make(chan int, len(usages))

	// Start progress updater in a separate goroutine
	if showProgress {
		go func() {
			for range progressUpdates {
				bar.Add(1)
			}
		}()
	}

	// Start worker goroutines
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				if usages[job.index].IsUsed {
					if showProgress {
						progressUpdates <- 1
					}
					continue
				}

				key := job.key

				// For security context and dict patterns, do a more thorough check
				for _, patternName := range f.Registry.Names {
					isUsed, parentKey, locations := f.isKeyUsedWithPattern(key, patternName)
					if isUsed {
						usages[job.index].IsUsed = true
						usages[job.index].UsageType = "pattern"
						usages[job.index].Locations = locations
						usages[job.index].ParentKey = parentKey
						usages[job.index].PatternName = patternName
						usedKeysMutex.Lock()
						usedKeysMap[key] = true
						usedKeysMutex.Unlock()
						break
					}
				}

				if showProgress {
					progressUpdates <- 1
				}
			}
		}()
	}

	// Send jobs for second pass
	for i, usage := range usages {
		jobs <- workItem{index: i, key: usage.Key}
	}

	// Close jobs channel for second pass
	close(jobs)

	// Wait for all second pass workers to finish
	wg.Wait()

	// Close progress updates channel for second pass
	if showProgress {
		close(progressUpdates)
	}

	return usages
}

