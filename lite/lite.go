package lite

type DB struct {
}

func New(path string) (db *DB, err error) {
	return
}

func (db *DB) Open() (err error) { return }

func (db *DB) Close() (er error) { return }

func (db *DB) Create(path string) (err error) { return }
