FROM ubuntu

# Add the PostgreSQL PGP key to verify their Debian packages.
# It should be the same key as https://www.postgresql.org/media/keys/ACCC4CF8.asc
RUN apt-key adv --keyserver hkp://p80.pool.sks-keyservers.net:80 --recv-keys B97B0AFCAA1A47F044F244A07FCC7D46ACCC4CF8

# Add PostgreSQL's repository. It contains the most recent stable release
#     of PostgreSQL, ``9.3``.
RUN echo "deb http://apt.postgresql.org/pub/repos/apt/ precise-pgdg main" > /etc/apt/sources.list.d/pgdg.list

# Install ``python-software-properties``, ``software-properties-common`` and PostgreSQL 9.3
#  There are some warnings (in red) that show up during the build. You can hide
#  them by prefixing each apt-get statement with DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y python-software-properties software-properties-common postgresql-9.3 postgresql-client-9.3 postgresql-contrib-9.3

# Note: The official Debian and Ubuntu images automatically ``apt-get clean``
# after each ``apt-get``

# Run the rest of the commands as the ``postgres`` user created by the ``postgres-9.3`` package when it was ``apt-get installed``
USER postgres

# Create a PostgreSQL role named ``trial3`` with ``trial3`` as the password and
# then create a database `trial3` owned by the ``trial3`` role.
RUN    /etc/init.d/postgresql start &&\
    psql --command "CREATE USER trial3 WITH SUPERUSER PASSWORD 'trial3';" &&\
    createdb -O trial3 trial3 &&\
    psql -d trial3 --command "CREATE TABLE trial3 (id SERIAL PRIMARY KEY, access_token TEXT NOT NULL, event_code TEXT NOT NULL, stream_type TEXT NOT NULL, sent_to TEXT NOT NULL, msg_data JSON);"

## Adjust PostgreSQL configuration so that remote connections to the database are possible.
RUN echo "host all all 0.0.0.0/0  md5" >> /etc/postgresql/9.3/main/pg_hba.conf

## And add ``listen_addresses`` to ``/etc/postgresql/9.3/main/postgresql.conf``.
RUN echo "listen_addresses='*'" >> /etc/postgresql/9.3/main/postgresql.conf

# Add VOLUMEs to allow backup of config, logs and databases.
VOLUME  ["/etc/postgresql", "/var/log/postgresql", "/var/lib/postgresql"]

# Set the default command to run when starting the container.
CMD ["/usr/lib/postgresql/9.3/bin/postgres", "-D", "/var/lib/postgresql/9.3/main", "-c", "config_file=/etc/postgresql/9.3/main/postgresql.conf"]