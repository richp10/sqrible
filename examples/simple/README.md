1. Import `./users.sql` in postgres
2. Build sqrible : `make -C ../../ build`
2. Rune: `PGDATABASE=yourdb PGHOST=localhost ../../bin/sqrible -c ./sqrible.yml -d . -t users`
