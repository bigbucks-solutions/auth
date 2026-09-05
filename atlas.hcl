data "external_schema" "gorm" {
  // The schema spans ./models and ./subscriptions. The atlas-provider-gorm CLI
  // accepts only one --path and emits just that package, so a local loader is
  // used instead; see tools/atlasloader for why. New model packages must be
  // registered there or a diff will generate DROP statements for them.
  program = [
    "go",
    "run",
    "-mod=mod",
    "./tools/atlasloader",
  ]
}

env "gorm" {
  src = data.external_schema.gorm.url
  dev = "docker://postgres/15/dev?search_path=public"
  migration {
    dir = "file://migrations"
    format = golang-migrate
  }
  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }

}
