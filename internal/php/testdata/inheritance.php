<?php

namespace App\Entity;

use Doctrine\ORM\Mapping as ORM;
use Traversable;
use Countable;
use App\BaseClass;

/**
 * A test class that extends a base class and implements interfaces
 */
#[ORM\Entity]
class Product extends BaseClass implements Traversable, Countable
{
    #[ORM\Id]
    private int $id;

    #[ORM\Column]
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

    // Implementation of Countable interface
    public function count(): int
    {
        return 1;
    }
}
