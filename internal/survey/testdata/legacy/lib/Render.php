<?php

# Renders output.
class Render
{
    function text($text)
    {
        return $this->block($text);
    }

    protected function block($line)
    {
        $method = 'block'.$line;
        return $this->$method($line);
    }

    private function report($v) { return parse($v); }
}
