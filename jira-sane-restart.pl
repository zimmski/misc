#!/usr/bin/perl

use strict;
use warnings;
use utf8;
use Encode qw(encode_utf8);

use File::Basename;
use File::Slurp;

#
# rcjira restart
#

print "Restart Jira:\n";

my $rc_jira_restart_out = system_exec("/usr/sbin/rcjira restart");

#
# sleep to the let service restart settle
#
sleep(5);

#
# checkk: rcjira status
#

my $rc_jira_status_out = system_exec("/usr/sbin/rcjira status");

if ($rc_jira_status_out =~ m/Active: active \(exited\)/s) {
	print "OK ... Jira Restart\n";
}
else {
	print "FAILED ... Jira Restart\n";
	print "\n$rc_jira_status_out\n";

	exit 1;
}

#
# sleep to the let asynchronous Jira start execute
#
sleep(60 * 4);

#
# check: Jira log
#

my $failed;
my $log = read_file("/srv/jira/bin/logs/catalina.out");

# only parse the last Jira start
$log =~ s/.+(JIRA starting.+?)/$1/s;

print "\nChecking Jira Log:\n";

for my $check (
	'JIRA starting',
	'Running JIRA startup checks',
	'JIRA pre-database startup checks completed successfully',
	'JIRA database startup checks completed successfully',
	'Database configuration OK',
	'Initialising the plugin system',
	'Plugin System Started',
	'You can now access JIRA through your web browser',
	'JIRA Scheduler started',
	'Server startup in \d+ ms'
) {
	if ($log =~ m/($check)/g) {
		print "OK ... $1\n";
	}
	else {
		print "FAILED ... $check\n";

		$failed = 1;
	}
}

# exceptions
my @exceptions = $log =~ m/^(.*exception.*)$/mgi;
if (@exceptions) {
	print "\nFound exceptions:\n";
	print "- $_\n" for @exceptions;
}

# errors
my @errors;

for my $i ($log =~ m/^(.*error.*)$/mgi) {
	$i =~ s/^\s+//;
	$i =~ s/\s+$//;

	if ($i and (
		$i !~ m/^jira.projectkey.warning\s+:\s+admin.errors.must.specify.unique.project.key$/
	)) {
		push(@errors, $i);
	}
}

if (@errors) {
	print "\nFound errors:\n";
	print "- $_\n" for @errors;
}

# disabling
my @disables = $log =~ m/^(.*disabling.*)$/mgi;
if (@disables) {
	print "\nFound disabled modules/plugins:\n";
	print "- $_\n" for @disables;
}

if ($failed) {
	exit 1;
}

#
# exit with sane state
#

exit 0;

sub system_exec {
	my ($cmd) = @_;

	my $out = `$cmd 2>&1`;

	if ($? != 0) {
		die "failed to execute rcjira restart:\n\n$out\n";
	}

	return $out;
}
