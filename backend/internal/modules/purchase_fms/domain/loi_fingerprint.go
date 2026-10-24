package domain

import "fmt"

func (d LOIDocument) Filename() string { return fmt.Sprintf("LOI-%s-v%d.pdf", d.ID[:8], d.Version) }
