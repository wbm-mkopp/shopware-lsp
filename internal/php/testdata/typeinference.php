<?php

namespace App\Entity;

class Product
{
    private int $id;
    private string $name;

    public function getId(): int
    {
        return $this->id;
    }

    public function getName(): string
    {
        return $this->name;
    }

    public function setName(string $name): self
    {
        $this->name = $name;
        return $this;
    }

    public function getNameWithPrefix(string $prefix): string
    {
        return $prefix . $this->getName();
    }

    public function getSelf(): self
    {
        return $this;
    }
}
