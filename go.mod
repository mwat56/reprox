module github.com/mwat56/reprox

go 1.23.0

toolchain go1.23.5

require (
	github.com/mwat56/apachelogger v1.7.0
	github.com/mwat56/ini v1.9.0
	github.com/mwat56/sourceerror v0.2.1
	golang.org/x/sys v0.31.0
)

replace (
	github.com/mwat56/apachelogger => ../apachelogger
	github.com/mwat56/cssfs => ../cssfs
	github.com/mwat56/errorhandler => ../errorhandler
	github.com/mwat56/hashtags => ../hashtags
	github.com/mwat56/ini => ../ini
	github.com/mwat56/jffs => ../jffs
	github.com/mwat56/pageview => ../pageview
	github.com/mwat56/passlist => ../passlist
	github.com/mwat56/screenshot => ../screenshot
	github.com/mwat56/sessions => ../sessions
	github.com/mwat56/uploadhandler => ../uploadhandler
	github.com/mwat56/whitespace => ../whitespace
)
