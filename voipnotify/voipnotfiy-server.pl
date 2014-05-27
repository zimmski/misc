#!/usr/bin/perl

use strict;
use warnings;
use utf8;
use Encode qw(encode_utf8);

use File::Basename;
use File::Copy;
use File::Slurp;
use Text::CSV::Slurp;

my $REVISION = 12;

my $script_path = dirname(__FILE__);

{
	package MyWebServer;

	use HTTP::Server::Simple::CGI;
	use base qw(HTTP::Server::Simple::CGI);

	my %allowed_receivers = ();

	my %dispatch = (
		'/hello' => \&response_hello,
		'/notification' => \&response_notification,
		'/update_receivers' => \&response_update_receivers,
	);

	sub handle_request {
		my ($self, $cgi) = @_;

		my $path = $cgi->path_info();

		if (exists $dispatch{$path}) {
			print "HTTP/1.0 200 OK\r\n";

			$dispatch{$path}->($cgi);
		}
		else {
			print "HTTP/1.0 404 Not found\r\n";
			print $cgi->header,
				$cgi->start_html('Not found'),
				$cgi->h1('Not found'),
				$cgi->end_html;
		}
	}

	sub response_hello {
		my ($cgi) = @_;

		return if !ref $cgi;

		my $who = $cgi->param('name');

		$who ||= 'no-name-defined';

		print $cgi->header,
			$cgi->start_html('Hello'),
			$cgi->h1("Hello $who! We are running revision $REVISION."),
			$cgi->end_html;
	}

	sub response_notification {
		my ($cgi) = @_;

		return if !ref $cgi;

		response_update_receivers('fake');

		my $message = $cgi->param('message');
		my @receivers = $cgi->param('receiver');
		my @numbers;
		my $notified_text;

		for my $i(@receivers) {
			if (exists $allowed_receivers{$i}) {
				$notified_text .= ($notified_text ? ', ' : '') . $i . '(' . $allowed_receivers{$i} . ')';
				push(@numbers, $allowed_receivers{$i});
			}
		}

		my $file_name ='monitoring-message';
		my $file = "$script_path/calls/$file_name";

		my $old_file_content = (-f "$file.txt") ? File::Slurp::read_file("$file.txt") : undef;

		if (not $old_file_content or $old_file_content ne $message) {
			File::Slurp::write_file("$file.txt", $message);
			`swift -n Allison-8KHz -f $file.txt -o $file.tmp.wav`;
			`sox $file.tmp.wav $file.wav trim 00:08.2 speed 0.95`;
		}

		`sox $file.wav -r 8000 -c 1 $file.gsm`;

		if (@numbers) {
			for my $number (@numbers) {
				$number =~ s/[\r\n]//sg;

				File::Slurp::write_file("$script_path/calls/monitoring-call-$number.call",
"Channel: SIP/trunk_1/$number
Application: Playback
Data: $file_name
");
			}

			print $cgi->header,
				$cgi->start_html('Sent notifications'),
				$cgi->h1('Notified ' . $notified_text),
				$cgi->end_html;
		}
		else {
			print $cgi->header,
				$cgi->start_html('Sent notifications'),
				$cgi->h1('Notified NOBODY! There is something wrong with your receivers.'),
				$cgi->end_html;
		}
	}

	sub response_update_receivers {
		my ($cgi, $fake) = @_;

		return if !ref $cgi;

		my $data = Text::CSV::Slurp->load(file => $script_path . '/allowed_receivers.csv');

		my %tmp = ();

		for my $i(@$data) {
			$tmp{$i->{name}} = $i->{number};
		}

		%allowed_receivers = %tmp;

		if (not $fake) {
			print $cgi->header,
				$cgi->start_html('Updated receivers'),
				$cgi->h1('Updated');

			print $cgi->start_ul;

			for my $i(sort keys %allowed_receivers) {
				print $cgi->li($i . '(' . $allowed_receivers{$i} . ')');
			}

			print $cgi->end_ul;

			print $cgi->end_html;
		}
	}
}

if (@ARGV and $ARGV[0] =~ m/^\d+$/s) {
	my $server = MyWebServer->new($ARGV[0]);

	$server->response_update_receivers('fake');

	my $pid = $server->background();

	print "Use 'kill $pid' to stop server.\n";
}
else {
	print "Start with\n\tserver.pl <port number>\n";
}
