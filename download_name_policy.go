package libupdate

import "time"

type NamePolicy interface {
	GenerateName() (string, error)
}

type TimeNamePolicy struct {
	Format string
}

func (this *TimeNamePolicy) GenerateName() (string, error) {
	result := time.Now().Format(this.Format)
	return result, nil
}
