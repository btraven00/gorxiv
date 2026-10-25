# gorxiv

Search [Europe PMC](https://europepmc.org) (articles and preprints) from the terminal, inspect a
record's metadata, and download its PDF when one is openly available.

## Install

    make build          # -> bin/gorxiv
    make install        # go install into $GOBIN

## Usage

    # search; Europe PMC query syntax passes through, flags are ANDed on
    gorxiv search 'perturb-seq AND "cancer cell lines"' --preprints --from 2025 -n 10
    gorxiv search --author "Dempster JM" --sort date --ids     # bare IDs, pipeable
    gorxiv search 'TITLE:"mix-seq"' --json                      # raw core records

    # metadata for a PMID, PMCID, PPR id or DOI
    gorxiv show 10.64898/2026.08.24.746802

    # PDF into a directory
    gorxiv pdf PMC7453022 --out papers/

Search flags: `--preprints` (SRC:PPR), `--oa` (OPEN_ACCESS:y), `--from/--to` (YYYY, YYYY-MM or
YYYY-MM-DD on FIRST_PDATE), `--author`, `--sort relevance|date|cited`, `-n/--limit` (cursor paging),
`--print-query` to see the composed query.

## What `pdf` tries

Only openly available full text, in order, keeping the first response that starts with `%PDF-`:

1. Europe PMC render, `https://europepmc.org/articles/PMCxxx?pdf=render` (PMCID only). Currently
   behind a bot-challenge page for scripted clients; gorxiv reports that and moves on.
2. NCBI's PMC Article Datasets on AWS (`pmc-oa-opendata`, open-access and author-manuscript
   subsets), latest version of the article (PMCID only).
3. `fullTextUrlList` entries with `documentStyle=pdf`, except "Subscription required".
4. Free/open DOI links: a direct PDF, or for a bioRxiv/medRxiv landing page, its `.full.pdf`.

The file is written atomically as `<id>.pdf` (DOIs slugged) and gorxiv prints path, size and
sha256. If nothing works it lists each URL tried with the reason and exits non-zero.
