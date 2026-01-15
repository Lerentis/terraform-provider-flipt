# (OPTIONAL) Overrides the copywrite config schema version
# Default: 1
schema_version = 1

project {
  # (OPTIONAL) SPDX-compatible license identifier
  # Leave blank if you don't wish to license the project
  # Default: "MPL-2.0"
  license = "MIT"

  # (OPTIONAL) Represents the copyright holder used in all statements
  # Default: IBM Corp.
  copyright_holder = "terraform-provider-flipt contributors"

  # (OPTIONAL) Represents the year that the project initially began
  # This is used as the starting year in copyright statements
  # If set and different from current year, headers will show: "copyright_year, current_year"
  # If set and same as current year, headers will show: "current_year"
  # If not set (0), it will be auto-detected from GitHub or use current year only
  # Default: <the year the repo was first created>
  # copyright_year = 0

  # (OPTIONAL) A list of globs that should not have copyright or license headers .
  # Supports doublestar glob patterns for more flexibility in defining which
  # files or folders should be ignored
  # Default: []
  header_ignore = [
    # internal catalog metadata (prose)
    "META.d/**/*.yaml",

    # examples used within documentation (prose)
    "examples/**",

    # GitHub issue template configuration
    ".github/ISSUE_TEMPLATE/*.yml",

    # golangci-lint tooling configuration
    ".golangci.yml",

    # GoReleaser tooling configuration
    ".goreleaser.yml",
  ]


  # (OPTIONAL) Links to an upstream repo for determining repo relationships
  # This is for special cases and should not normally be set.
  # Default: ""
  # upstream = "hashicorp/<REPONAME>"
}
