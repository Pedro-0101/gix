//go:build !windows

package winnotify

// noopNotifier: fora do Windows não há agendamento nativo; o push do servidor
// continua sendo o único caminho.
type noopNotifier struct{}

func New() Notifier { return noopNotifier{} }

func (noopNotifier) Arm(Occurrence) error          { return nil }
func (noopNotifier) CancelByAlert(int64) error      { return nil }
func (noopNotifier) ListArmed() ([]Key, error)      { return nil, nil }
