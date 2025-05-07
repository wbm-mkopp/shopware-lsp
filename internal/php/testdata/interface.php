<?php

namespace App\Interfaces;

use Traversable;
use App\Contracts\LoggerInterface;

/**
 * A test interface that extends other interfaces
 */
interface CustomInterface extends Traversable, LoggerInterface
{
    /**
     * Get the custom value
     *
     * @return string
     */
    public function getCustomValue(): string;

    /**
     * Set the custom value
     *
     * @param string $value
     * @return self
     */
    public function setCustomValue(string $value): self;
}
