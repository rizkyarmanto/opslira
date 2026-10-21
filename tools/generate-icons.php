<?php

/**
 * Regenerates the favicon set from public/images/opslira.jpg.
 *
 * Run: php tools/generate-icons.php
 *
 * The source is a square JPEG, but the crop is computed rather than assumed so a
 * future logo export with a different aspect ratio still produces square icons.
 */

declare(strict_types=1);

$source = __DIR__.'/../public/images/opslira.jpg';

if (! function_exists('imagecreatefromjpeg')) {
    throw new RuntimeException('The GD extension is not available: imagecreatefromjpeg() is missing.');
}

if (! is_file($source)) {
    throw new RuntimeException("Source logo not found: {$source}");
}

$src = @imagecreatefromjpeg($source);
if ($src === false) {
    throw new RuntimeException("Unable to read the source JPEG: {$source}");
}

$srcW = imagesx($src);
$srcH = imagesy($src);
$side = min($srcW, $srcH);
$cropX = (int) (($srcW - $side) / 2);
$cropY = (int) (($srcH - $side) / 2);

/**
 * Center-cropped square resize.
 */
$render = static function (int $size) use ($src, $cropX, $cropY, $side) {
    $out = imagecreatetruecolor($size, $size);
    if ($out === false) {
        throw new RuntimeException("Unable to create a {$size}x{$size} canvas.");
    }
    imagecopyresampled($out, $src, 0, 0, $cropX, $cropY, $size, $size, $side, $side);

    return $out;
};

$targets = [
    'favicon-32.png' => [32, 9],
    'apple-touch-icon.png' => [180, 9],
    'favicon-192.png' => [192, 9],
];

$png32 = null;

foreach ($targets as $filename => [$size, $quality]) {
    $image = $render($size);
    $path = __DIR__.'/../public/'.$filename;

    if (! @imagepng($image, $path, $quality)) {
        imagedestroy($image);
        throw new RuntimeException("Unable to write {$path}");
    }

    if ($filename === 'favicon-32.png') {
        $png32 = file_get_contents($path);
    }

    imagedestroy($image);
    echo "wrote {$filename} ({$size}x{$size})\n";
}

if ($png32 === false || $png32 === null) {
    throw new RuntimeException('The 32x32 PNG payload could not be read back for favicon.ico.');
}

/**
 * ICO container: 6-byte ICONDIR, one 16-byte ICONDIRENTRY, then the PNG payload.
 * Modern browsers accept a PNG image inside an ICO, so no other ICO writer is needed.
 */
$iconDir = pack('vvv', 0, 1, 1);
$entry = pack(
    'CCCCvvVV',
    32,                // width
    32,                // height
    0,                 // color count, 0 for truecolor
    0,                 // reserved
    1,                 // planes
    32,                // bits per pixel
    strlen($png32),    // bytes in resource
    22                 // offset: 6-byte header + 16-byte entry
);

$icoPath = __DIR__.'/../public/favicon.ico';
if (@file_put_contents($icoPath, $iconDir.$entry.$png32) === false) {
    throw new RuntimeException("Unable to write {$icoPath}");
}
echo "wrote favicon.ico (32x32 PNG payload)\n";

imagedestroy($src);
