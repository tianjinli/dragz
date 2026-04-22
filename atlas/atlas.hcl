data "external_schema" "gorm" {
  program = [
    "go",
    "run",
    "-mod=mod",
    "ariga.io/atlas-provider-gorm",
    "load",
    "--path", "../internal/entity",
    "--dialect", "${getenv("ATLAS_DIAL")}",
  ]
}

# atlas migrate diff [name] --env gorm
# ATLAS_DIAL=postgres ATLAS_DEV="docker://postgres/18-alpine/?search_path=public&sslmode=disable" atlas migrate diff --env gorm
# ATLAS_DIAL=mysql ATLAS_DEV="docker://mysql/8-debian/" atlas migrate diff --env gorm
env "gorm" {
  src = data.external_schema.gorm.url

  url = "${getenv("ATLAS_URL")}"
  dev = "${getenv("ATLAS_DEV")}"

  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }

  diff {
    skip {
      drop_schema = true
    }
  }
}
