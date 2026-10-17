<?php
// Characterisation driver: reads Markdown on stdin, prints Parsedown's HTML.
require __DIR__ . '/Parsedown.php';
$p = new Parsedown();
$line = false;
foreach (array_slice($argv, 1) as $a) {
    switch ($a) {
        case '--safe': $p->setSafeMode(true); break;
        case '--strict': $p->setStrictMode(true); break;
        case '--breaks': $p->setBreaksEnabled(true); break;
        case '--escaped': $p->setMarkupEscaped(true); break;
        case '--no-urls': $p->setUrlsLinked(false); break;
        case '--line': $line = true; break;
        default: fwrite(STDERR, "unknown flag $a\n"); exit(2);
    }
}
$in = stream_get_contents(STDIN);
echo $line ? $p->line($in) : $p->text($in);
