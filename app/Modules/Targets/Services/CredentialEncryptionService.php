<?php

namespace App\Modules\Targets\Services;

use RuntimeException;

/**
 * AES-256-GCM credential encryption.
 * Stores ciphertext + IV + auth tag separately (target_credentials table).
 */
class CredentialEncryptionService
{
    private string $key;

    private string $cipher = 'aes-256-gcm';

    public function __construct()
    {
        $key = (string) env('APP_ENCRYPTION_KEY', '');
        if ($key === '') {
            // Fallback to app key so local dev works; production must set APP_ENCRYPTION_KEY.
            $key = (string) config('app.key');
            $key = str_starts_with($key, 'base64:') ? base64_decode(substr($key, 7)) : $key;
        }

        $this->key = hash('sha256', $key, true);
    }

    /**
     * @param  array<string, mixed>  $payload
     * @return array{encrypted_payload: string, encryption_iv: string, encryption_tag: string}
     */
    public function encrypt(array $payload): array
    {
        $ivLength = openssl_cipher_iv_length($this->cipher);
        $iv = random_bytes($ivLength);
        $tag = '';

        $ciphertext = openssl_encrypt(
            json_encode($payload, JSON_THROW_ON_ERROR),
            $this->cipher,
            $this->key,
            OPENSSL_RAW_DATA,
            $iv,
            $tag,
        );

        if ($ciphertext === false) {
            throw new RuntimeException('Credential encryption failed');
        }

        return [
            'encrypted_payload' => base64_encode($ciphertext),
            'encryption_iv' => base64_encode($iv),
            'encryption_tag' => base64_encode($tag),
        ];
    }

    /**
     * @return array<string, mixed>
     */
    public function decrypt(string $payload, string $iv, string $tag): array
    {
        $plaintext = openssl_decrypt(
            base64_decode($payload),
            $this->cipher,
            $this->key,
            OPENSSL_RAW_DATA,
            base64_decode($iv),
            base64_decode($tag),
        );

        if ($plaintext === false) {
            throw new RuntimeException('Credential decryption failed');
        }

        return json_decode($plaintext, true, 512, JSON_THROW_ON_ERROR);
    }
}
