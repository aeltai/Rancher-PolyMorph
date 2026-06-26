package backup

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aeltai/rancher-polymorph/internal/ui"
)

func Sanitize(opts Options) (*Result, error) {
	log, err := ui.NewLogger(opts.Quiet, opts.Verbose, opts.LogFile)
	if err != nil {
		return nil, err
	}
	defer log.Close()

	info, err := os.Stat(opts.Input)
	if err != nil {
		return nil, err
	}

	level := compressLevel(opts)
	start := time.Now()

	log.Step("reading backup index from %s (%s)", opts.Input, HumanSize(info.Size()))

	headers, err := readAllHeaders(opts.Input)
	if err != nil {
		return nil, err
	}

	allNames := make([]string, len(headers))
	for i, h := range headers {
		allNames[i] = h.Name
	}
	log.Debug("indexed %d tar members", len(headers))

	progress := ui.NewProgressBar(len(headers), "Sanitize", !opts.Quiet, log.LogWriter())

	clusters, fleetIndex, err := buildInventory(opts.Input, headers)
	if err != nil {
		return nil, err
	}

	log.Info("inventory: %d cluster(s), %d fleet mapping(s)", len(clusters), len(fleetIndex.NameToCluster))

	removeIDs, autoOrphans := buildRemovePlan(opts, clusters, fleetIndex, allNames)
	matcher := NewRemovalMatcher(removeIDs, fleetIndex)

	result := &Result{
		InputPath:     opts.Input,
		OutputPath:    opts.Output,
		InputSize:     info.Size(),
		CompressLevel: level,
		Clusters:      clusters,
		FleetMappings: len(fleetIndex.NameToCluster),
		RemoveIDs:     removeIDs,
		AutoOrphans:   autoOrphans,
	}

	explicit := make([]string, 0)
	autoSet := make(map[string]struct{}, len(autoOrphans))
	for _, o := range autoOrphans {
		autoSet[o] = struct{}{}
	}
	for _, cid := range opts.RemoveClusters {
		if _, ok := autoSet[cid]; !ok {
			explicit = append(explicit, cid)
		}
	}
	sort.Strings(explicit)
	result.ExplicitRemove = explicit

	removeList := make([]string, 0, len(removeIDs))
	for id := range removeIDs {
		removeList = append(removeList, id)
	}
	sort.Strings(removeList)
	if len(removeList) > 0 {
		log.Info("removal plan: %d cluster ID(s): %s", len(removeList), stringsJoinLimited(removeList, 8))
	}
	if len(autoOrphans) > 0 {
		log.Warn("auto-detected %d orphan ID(s): %s", len(autoOrphans), strings.Join(autoOrphans, ", "))
	}

	if !opts.InspectOnly {
		if err := os.MkdirAll(filepath.Dir(opts.Output), 0o755); err != nil {
			return nil, err
		}
	}

	inFile, err := os.Open(opts.Input)
	if err != nil {
		return nil, err
	}
	defer inFile.Close()
	gr, err := gzip.NewReader(inFile)
	if err != nil {
		return nil, err
	}
	defer gr.Close()
	tr := tar.NewReader(gr)

	var outFile *os.File
	var gw *gzip.Writer
	var tw *tar.Writer
	if !opts.InspectOnly {
		outFile, err = os.Create(opts.Output)
		if err != nil {
			return nil, err
		}
		defer outFile.Close()
		gw, err = gzip.NewWriterLevel(outFile, level)
		if err != nil {
			return nil, err
		}
		defer gw.Close()
		tw = tar.NewWriter(gw)
		defer tw.Close()
	}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		var doc map[string]any
		var body []byte
		if hdr.Size > 0 && shouldParseJSON(hdr.Name) {
			body = make([]byte, hdr.Size)
			if _, err := io.ReadFull(tr, body); err != nil {
				return nil, err
			}
			doc = parseJSONIfNeeded(body)
		} else if hdr.Size > 0 {
			body = make([]byte, hdr.Size)
			if _, err := io.ReadFull(tr, body); err != nil {
				return nil, err
			}
		}

		reason := matcher.Reason(hdr.Name, doc)
		if reason != "" {
			result.Removed = append(result.Removed, RemovedEntry{Path: hdr.Name, Reason: reason})
		} else if opts.InspectOnly {
			result.Kept = append(result.Kept, hdr.Name)
			result.KeptBytes += hdr.Size
		} else if hdr.Typeflag == tar.TypeDir || hdr.Typeflag == tar.TypeSymlink || hdr.Typeflag == tar.TypeLink {
			if err := tw.WriteHeader(hdr); err != nil {
				return nil, err
			}
			result.Kept = append(result.Kept, hdr.Name)
		} else if len(body) == 0 && hdr.Size == 0 {
			if err := tw.WriteHeader(hdr); err != nil {
				return nil, err
			}
			result.Kept = append(result.Kept, hdr.Name)
		} else if hdr.Size > 0 {
			if err := tw.WriteHeader(hdr); err != nil {
				return nil, err
			}
			if _, err := tw.Write(body); err != nil {
				return nil, err
			}
			result.Kept = append(result.Kept, hdr.Name)
			result.KeptBytes += hdr.Size
		} else {
			result.Removed = append(result.Removed, RemovedEntry{Path: hdr.Name, Reason: "unreadable member"})
		}

		progress.Advance()
		if opts.ProgressFn != nil {
			opts.ProgressFn(progress.Current(), progress.Total())
		}
	}

	progress.Finish(fmt.Sprintf("Sanitize pass complete — kept %d, removed %d in %.1fs",
		len(result.Kept), len(result.Removed), time.Since(start).Seconds()))

	if !opts.InspectOnly {
		if err := tw.Close(); err != nil {
			return nil, err
		}
		if err := gw.Close(); err != nil {
			return nil, err
		}
		if err := outFile.Close(); err != nil {
			return nil, err
		}
		outInfo, err := os.Stat(opts.Output)
		if err != nil {
			return nil, err
		}
		result.OutputSize = outInfo.Size()
	}

	result.Elapsed = time.Since(start)
	if !opts.InspectOnly {
		log.OK("wrote %s (%s, gzip %d, %.1fs total)",
			opts.Output, HumanSize(result.OutputSize), level, result.Elapsed.Seconds())
	}

	return result, nil
}

func stringsJoinLimited(items []string, max int) string {
	if len(items) <= max {
		return strings.Join(items, ", ")
	}
	return strings.Join(items[:max], ", ") + fmt.Sprintf(", … +%d more", len(items)-max)
}
