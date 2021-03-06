**NOTE**: code & documentation are of beta quality at this stage, not for
production use.

Introduction
============

INTERLOCK
Copyright (c) 2015 Inverse Path S.r.l.

The INTERLOCK application is a file encryption front-end developed, but not
limited to, usage with the [USB armory](http://inversepath.com/usbarmory).

The goal of the package is to expose a web-based file manager for an encrypted
partition running on the device hosting the JSON application server (i.e. USB
armory).

The file manager allows uploading/downloading of files to/from the encrypted
partition, as well as additional symmetric/asymmetric cryptographic operations
on the individual files.

Architecture
============

The application provides a web application (client-side) and its counterpart
JSON application server implementing the protocol specified in the API
document.

The JSON application server is written in golang. The client HTML/Javascript
application is statically served by the application server, implementing the
presentation layer.

The interaction between the static HTML/Javascript and the JSON application
server is entirely documented in the API document.

The authentication is directly tied to Linux Unified Key Setup (LUKS)
disk-encryption setup on the server side. A successful login unlocks the
specified encrypted volume, while logging out locks it back.

Design goals:

* Clear separation between presentation and server layer to ease auditability
  and integration.

* Minimum amount of external dependencies, currently no code outside of Go
  standard and supplementary libraries is required for the server binary.

* Authentication process directly tied to LUKS partition locking/unlocking.

* Support for additional symmetric/asymmetric encryption on individual
  files/directories.

* Minimize exposure of sensitive data to the client with support for disposable
  authentication passwords, server-side operations (key generation,
  encryption/decryption, archive creation/extraction) and locking of private
  keys.

* Minimal footprint (single statically linked binary + supporting static files)
  to ease integration with USB armory secure booted initrd ramdisk.

Supported Ciphers
=================

Encrypted volumes:

* LUKS encrypted partitions

Asymmetric ciphers:

* OpenPGP (using golang.org/x/crypto/openpgp)

Symmetric ciphers:

* AES-256-OFB w/ PBKDF2 password derivation (SHA256, 4096 rounds) and HMAC (SHA256)

Security tokens:

* Time-based One-Time Password Algorithm (TOTP), RFC623 implementation (Google Authenticator)

Key Storage
===========

A pre-defined directory, stored on the encrypted filesystem, is assigned to
public and private key storage (see Configuration section for related
settings).

The keys can be uploaded using the file manager, imported as free text or
generated server-side.

The key storage directory structure is the following:

```
<key_path>/<private|public>/<cipher_name>/<key_identifier>.<key_format>
```

Once uploaded in their respective directory, private keys can only be deleted
or overwritten, they cannot be downloaded, moved or copied.

The keys for OTP ciphers (e.g. "TOTP" implementing Google Authenticator)
generate a valid OTP code, for the current time, when the key information is
queried ('Key Info' action on the right click menu).

Requirements & Operation
========================

The use of INTERLOCK is coupled with the presence of at least one LUKS
encrypted partition, its initial creation is pre-requisite left to the user.

An example setup using cryptsetup and LVM2 follows. The example uses a microSD
partition to illustrate typical USB armory setup, the partition (mmcblk0p2) is
assumed to have been previously created with fdisk using the desired size and
the Linux LVM type (8e).

```
pvcreate /dev/mmcblk0p2                   # initialize physical volume
vgcreate lvmvolume /dev/mmcblk0p2         # create volume group
lvcreate -L 20G -n encryptedfs lvmvolume  # create logical volume of 20 GB

cryptsetup -y --cipher aes-xts-plain64  \ # set-up encrypted partition
  --key-size 256 --hash sha1 luksFormat \ # with default cryptsetup
  /dev/lvmvolume/encryptedfs              # settings

cryptsetup luksOpen /dev/lvmvolume/encryptedfs interlockfs
mkfs.ext4 /dev/mapper/interlockfs         # create ext4 filesystem
cryptsetup luksClose interlockfs
```

The login procedure of INTERLOCK prompts for an encrypted volume name (e.g.
encryptedfs in the previous example) and one valid password for luksOpen.

A successful login unlocks the encrypted partition, a successful logout locks
it back.

Once logged in users can change, add, remove LUKS passwords within INTERLOCK.
Any login password can be disposed of using a dedicated flag during login, this
deletes the password from its LUKS key slot right after encrypted partition
unlocking.

**WARNING**: removing the last remaining password makes the LUKS encrypted
container permanently inaccessible. This is a feature, not a bug.

The following sudo configuration (meant to be included in /etc/sudoers)
illustrates the permission requirements for the user running the INTERLOCK
server. The example assumes username 'interlock' with home directory
'/home/interlock'.

```
interlock ALL=(root) NOPASSWD:							\
	/bin/date -s @*,							\
	/sbin/poweroff,								\
	/bin/mount /dev/mapper/interlockfs /home/interlock/.interlock-mnt,	\
	/bin/umount /home/interlock/.interlock-mnt,				\
	/bin/chown interlock /home/interlock/.interlock-mnt,			\
	/sbin/cryptsetup luksOpen /dev/lvmvolume/* interlockfs,			\
	!/sbin/cryptsetup luksOpen /dev/lvmvolume/*.* *,			\
	/sbin/cryptsetup luksClose /dev/mapper/interlockfs,			\
	!/sbin/cryptsetup luksClose /dev/mapper/*.*,				\
	/sbin/cryptsetup luksChangeKey /dev/lvmvolume/*,			\
	!/sbin/cryptsetup luksChangeKey /dev/lvmvolume/*.*,			\
	/sbin/cryptsetup luksRemoveKey /dev/lvmvolume/*,			\
	!/sbin/cryptsetup luksRemoveKey /dev/lvmvolume/*.*,			\
	/sbin/cryptsetup luksAddKey /dev/lvmvolume/*,				\
	!/sbin/cryptsetup luksAddKey /dev/lvmvolume/*.*
```

Compiling
=========

The INTERLOCK app requires a working Go (>= 1.4.2) environment to be compiled,
or cross-compiled, under Linux (it is not supported by or designed for other
OSes at this time).

```
go get -u golang.org/x/crypto/pbkdf2
go get -u golang.org/x/crypto/openpgp
git clone https://github.com/inversepath/interlock
cd interlock
make
```

This compiles the 'interlock' binary that can be executed with options
illustrated in the next section.

When cross compiling from a non-arm host for an arm target ensure that the
following compilation variables are set:

```
make GOARCH=arm \
     CC=<path_to_cross_compiler>/arm-linux-gnueabihf-gcc \
     CGO_ENABLED=1
```

Options
=======

```
  -h                   options help
  -b="127.0.0.1:443"   binding address:port pair
  -c="interlock.conf"  configuration file path
  -d=false:            debug mode
  -t=false:            test mode (WARNING: disables authentication)
```

Configuration
=============

* debug: enable debugging logs

* set_time: use the client browser time to set server time at login, useful on
  non-routed USB armory devices (unable to set the clock on their own)

* bind_address: IP address, port pair

* tls_cert: HTTPS server TLS certificate

* tls_key: HTTPS server TLS key

* key_path: path for public/private key storage on the encrypted filesystem

* volume_group: volume group name

* ciphers: array of cipher names to enable

The following example illustrates the configuration file format (plain JSON)
and defaults.

```
{
        "debug": false,
        "set_time": false,
        "bind_address": "127.0.0.1:4430",
        "tls_cert": "certs/cert.pem",
        "tls_key": "certs/key.pem",
        "key_path": "keys",
        "volume_group": "lvmvolume"
        "ciphers": [
                "OpenPGP",
                "AES-256-OFB",
                "TOTP"
        ]
}

```

At startup the interlock server dumps the applied configuration in its file
format.

Logging
=======

The application generates debug, audit, notification and error logs.

Debugging logs are only generated when "debug" is set to true in the
configuration file (or command line switch). In debug mode all logs are printed
on standard output and never saved.

Audit and error logs are shown live in a dedicated area on the web client
('Application logs') and saved on the root directory of the encrypted partition
in the '.interlock.log' file.

Notifications are shown live in a dedicated area on the web client ('Current
activity'), they are only kept in memory in a circular buffer and never stored
on disk.

Any non-debug log generated outside an unauthenticated session is issued
through standard syslog facility.

Authors
=======

Andrea Barisani <andrea@inversepath.com>  
Daniele Bianco  <danbia@inversepath.com>  

License
=======

INTERLOCK | https://github.com/inversepath/interlock
Copyright (c) 2015 Inverse Path S.r.l.

Permission to use, copy, modify, and distribute this software for any purpose
with or without fee is hereby granted, provided that the above copyright notice
and this permission notice appear in all copies.

THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES WITH
REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY AND
FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY SPECIAL, DIRECT,
INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM
LOSS OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR
OTHER TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR
PERFORMANCE OF THIS SOFTWARE.
